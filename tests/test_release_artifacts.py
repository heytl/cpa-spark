import hashlib,importlib.util,json,struct,sys,tempfile,unittest,zipfile
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'scripts'))
import release_artifacts as r

def fixture(target):
    os_name,arch=target.split('_');b=bytearray(128)
    if os_name=='linux':b[:6]=b'\x7fELF\x02\x01';struct.pack_into('<HH',b,16,3,62 if arch=='amd64' else 183)
    elif os_name=='darwin':b[:4]=b'\xcf\xfa\xed\xfe';struct.pack_into('<I',b,4,0x1000007 if arch=='amd64' else 0x100000c);struct.pack_into('<I',b,12,6)
    else:b[:2]=b'MZ';struct.pack_into('<I',b,0x3c,64);b[64:68]=b'PE\0\0';struct.pack_into('<H',b,68,0x8664);struct.pack_into('<H',b,86,0x2000)
    return bytes(b)

class ReleaseTests(unittest.TestCase):
    def make_native(self,root,target):
        folder=root/'native'/target;folder.mkdir(parents=True);raw=fixture(target);(folder/f'cpa-spark.{r.TARGETS[target]}').write_bytes(raw)
        (folder/'abi.json').write_text(json.dumps({'passed':True,'target':target,'sha256':hashlib.sha256(raw).hexdigest(),'version':r.source_version(),'id':'cpa-spark','name':'CPA Spark'}));return folder
    def test_complete_layout_hashes_and_targets(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d)
            for t in r.TARGETS:self.make_native(root,t)
            names=r.package(root/'native',root/'release',r.source_version(),list(r.TARGETS),True)
            self.assertEqual(len(names),5)
            for t in r.TARGETS:self.assertEqual(r.binary_target(r.verify(root/'release',r.source_version(),t)),t)
    def test_wrong_architecture_rejected(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);folder=self.make_native(root,'linux_arm64');(folder/'cpa-spark.so').write_bytes(fixture('linux_amd64'))
            with self.assertRaisesRegex(ValueError,'architecture'):r.package(root/'native',root/'release',r.source_version(),['linux_arm64'],True)
    def test_changed_binary_invalidates_receipt(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);folder=self.make_native(root,'linux_amd64');(folder/'cpa-spark.so').write_bytes(fixture('linux_amd64')+b'changed')
            with self.assertRaisesRegex(ValueError,'receipt'):r.package(root/'native',root/'release',r.source_version(),['linux_amd64'],True)
    def test_prerelease_not_store_latest(self):
        with self.assertRaisesRegex(ValueError,'numeric'):r.package('none','none','0.1.0-alpha.2',[],False)
    def test_missing_platform_fails_before_output(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);self.make_native(root,'linux_amd64')
            with self.assertRaises(FileNotFoundError):r.package(root/'native',root/'release',r.source_version(),list(r.TARGETS),True)
            self.assertFalse((root/'release').exists())
    def test_bad_checksum_and_nested_library_rejected(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);self.make_native(root,'linux_amd64');r.package(root/'native',root/'release',r.source_version(),['linux_amd64'],True)
            archive=root/'release'/f'cpa-spark_{r.source_version()}_linux_amd64.zip'
            with zipfile.ZipFile(archive,'w') as z:z.writestr('nested/cpa-spark.so',fixture('linux_amd64'))
            with self.assertRaisesRegex(ValueError,'checksum'):r.verify(root/'release',r.source_version(),'linux_amd64')
            (root/'release/checksums.txt').write_text(hashlib.sha256(archive.read_bytes()).hexdigest()+'  '+archive.name+'\n')
            with self.assertRaisesRegex(ValueError,'root-level'):r.verify(root/'release',r.source_version(),'linux_amd64')
if __name__=='__main__':unittest.main()
