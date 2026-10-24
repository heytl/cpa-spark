"""Create a missing version tag after CI; never move or overwrite an existing tag."""
import json
import os
from pathlib import Path
import re
from urllib.error import HTTPError
from urllib.request import Request, urlopen
from release_artifacts import source_version
from validate_tag import validate


def prepare(repository, sha, api):
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repository):
        raise ValueError('Invalid GitHub repository')
    if not re.fullmatch(r'[0-9a-f]{40}', sha):
        raise ValueError('Expected the checked-out commit SHA')
    tag = validate('v' + source_version())
    route = f'/repos/{repository}/git/ref/tags/{tag}'
    try:
        api('GET', route, None)
    except HTTPError as error:
        if error.code != 404:
            raise
    else:
        try:
            release = api('GET', f'/repos/{repository}/releases/tags/{tag}', None)
            resume = bool(release.get('draft'))
        except HTTPError as error:
            if error.code != 404:
                raise
            resume = True
        return {'created': 'false', 'resume': str(resume).lower(), 'tag': tag}
    api('POST', f'/repos/{repository}/git/refs', {'ref': 'refs/tags/' + tag, 'sha': sha})
    return {'created': 'true', 'resume': 'false', 'tag': tag}


def github_api(method, route, body):
    token = os.environ['GH_TOKEN']
    request = Request('https://api.github.com' + route,
                      data=None if body is None else json.dumps(body).encode(),
                      method=method,
                      headers={'Authorization': 'Bearer ' + token,
                               'Accept': 'application/vnd.github+json',
                               'Content-Type': 'application/json',
                               'User-Agent': 'cpa-spark-release',
                               'X-GitHub-Api-Version': '2022-11-28'})
    with urlopen(request, timeout=30) as response:
        return json.load(response)


if __name__ == '__main__':
    result = prepare(os.environ['GITHUB_REPOSITORY'], os.environ['GITHUB_SHA'], github_api)
    with Path(os.environ['GITHUB_OUTPUT']).open('a', encoding='utf-8', newline='\n') as output:
        for key, value in result.items():
            output.write(f'{key}={value}\n')
    print('Created tag' if result['created'] == 'true' else 'Existing tag retained', result['tag'])
