"""CPA Store platform packages. Source bundles are produced separately by package.py."""
import argparse, hashlib, json, re, struct, zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TARGETS = {'linux_amd64': 'so', 'linux_arm64': 'so', 'darwin_amd64': 'dylib', 'darwin_arm64': 'dylib', 'windows_amd64': 'dll'}
PLUGIN_ID = 'cpa-spark'

def source_version():
    return re.search(r'const Version = "([^"]+)"', (ROOT/'internal/warmup/types.go').read_text())[1]

def binary_target(raw):
    if raw[:4] == b'\x7fELF':
        if len(raw) < 64 or raw[4:6] != b'\x02\x01' or struct.unpack_from('<H', raw, 16)[0] != 3:
            raise ValueError('Expected little-endian ELF64 shared library')
        arch = {62:'amd64', 183:'arm64'}.get(struct.unpack_from('<H', raw, 18)[0])
        if arch:return 'linux_'+arch
    if raw[:4] == b'\xcf\xfa\xed\xfe' and len(raw) >= 32:
        arch = {0x1000007:'amd64', 0x100000c:'arm64'}.get(struct.unpack_from('<I',raw,4)[0])
        if arch and struct.unpack_from('<I',raw,12)[0] == 6:return 'darwin_'+arch
    if raw[:2] == b'MZ' and len(raw) >= 64:
        offset = struct.unpack_from('<I',raw,0x3c)[0]
        if offset+24 <= len(raw) and raw[offset:offset+4] == b'PE\0\0' and struct.unpack_from('<H',raw,offset+4)[0] == 0x8664 and struct.unpack_from('<H',raw,offset+22)[0] & 0x2000:return 'windows_amd64'
    raise ValueError('Unrecognized or unsupported dynamic library architecture')

def verify(directory, version, target):
    directory=Path(directory);name=f'{PLUGIN_ID}_{version}_{target}.zip'
    lines=(directory/'checksums.txt').read_text().splitlines();entries={}
    for line in lines:
        if not line:continue
        digest, filename=line.split('  ',1)
        if filename in entries or not re.fullmatch('[a-f0-9]{64}',digest):raise ValueError('Invalid or duplicate checksum')
        entries[filename]=digest
    raw=(directory/name).read_bytes()
    if entries.get(name)!=hashlib.sha256(raw).hexdigest():raise ValueError('Archive checksum mismatch')
    with zipfile.ZipFile(directory/name) as z:
        expected=f'{PLUGIN_ID}.{TARGETS[target]}'
        if z.namelist()!=[expected]:raise ValueError('ZIP must contain exactly one root-level plugin library')
        if z.testzip():raise ValueError('Archive CRC failed')
        binary=z.read(expected)
        if binary_target(binary)!=target:raise ValueError('Binary architecture mismatch')
    return binary

def package(native, output, version, targets, allow_prerelease=False):
    pattern=r'\d+\.\d+\.\d+'+(r'(?:-[A-Za-z0-9.-]+)?' if allow_prerelease else '')
    if not re.fullmatch(pattern,version):raise ValueError('Store latest requires numeric x.y.z; use --allow-prerelease only for development bundles')
    if version!=source_version():raise ValueError('Release version must match source Version')
    prepared=[]
    for target in targets:
        library=Path(native)/target/f'{PLUGIN_ID}.{TARGETS[target]}';raw=library.read_bytes()
        if binary_target(raw)!=target:raise ValueError(f'{target}: wrong binary architecture')
        receipt=json.loads(library.with_name('abi.json').read_text())
        if receipt.get('sha256')!=hashlib.sha256(raw).hexdigest() or receipt.get('version')!=version or receipt.get('target')!=target or receipt.get('id')!=PLUGIN_ID or receipt.get('name')!='CPA Spark' or receipt.get('passed') is not True:
            raise ValueError(f'{target}: matching native ABI test receipt required')
        prepared.append((target,raw))
    output=Path(output)
    if output.exists() and any(output.iterdir()):raise ValueError('Output directory must be empty to avoid mixing release assets')
    output.mkdir(parents=True,exist_ok=True);checks=[]
    for target,raw in prepared:
        name=f'{PLUGIN_ID}_{version}_{target}.zip'
        with zipfile.ZipFile(output/name,'w',zipfile.ZIP_DEFLATED) as z:z.writestr(f'{PLUGIN_ID}.{TARGETS[target]}',raw)
        checks.append(hashlib.sha256((output/name).read_bytes()).hexdigest()+'  '+name)
    (output/'checksums.txt').write_text('\n'.join(checks)+'\n')
    for target in targets:verify(output,version,target)
    return [line.split('  ')[1] for line in checks]

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--native',default='dist/native');p.add_argument('--output',required=True);p.add_argument('--version',default=source_version());p.add_argument('--target',action='append',choices=TARGETS);p.add_argument('--allow-prerelease',action='store_true');a=p.parse_args()
    print(json.dumps(package(a.native,a.output,a.version,a.target or list(TARGETS),a.allow_prerelease),indent=2))
