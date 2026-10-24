import sys
import unittest
from pathlib import Path
from urllib.error import HTTPError
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
from prepare_release import prepare
from release_artifacts import source_version


class PrepareReleaseTests(unittest.TestCase):
    def test_existing_tag_is_never_moved(self):
        calls = []
        def api(method, route, body):
            calls.append((method, route, body))
            return {'object': {'sha': 'b' * 40}}
        result = prepare('owner/repo', 'a' * 40, api)
        self.assertEqual(result, {'created': 'false', 'tag': 'v' + source_version()})
        self.assertEqual(len(calls), 1)
        self.assertEqual(calls[0][0], 'GET')

    def test_only_missing_tags_are_created(self):
        calls = []
        def api(method, route, body):
            calls.append((method, route, body))
            if method == 'GET':
                raise HTTPError(route, 404, 'Not Found', {}, None)
            return {}
        self.assertEqual(prepare('owner/repo', 'a' * 40, api)['created'], 'true')
        self.assertEqual(calls[-1], ('POST', '/repos/owner/repo/git/refs',
                                     {'ref': 'refs/tags/v' + source_version(), 'sha': 'a' * 40}))

    def test_permission_and_network_errors_do_not_create_tags(self):
        for code in (403, 500):
            calls = []
            def api(method, route, body):
                calls.append(method)
                raise HTTPError(route, code, 'Error', {}, None)
            with self.assertRaises(HTTPError):
                prepare('owner/repo', 'a' * 40, api)
            self.assertEqual(calls, ['GET'])

    def test_untrusted_inputs_are_rejected_before_api_calls(self):
        def api(*args):
            self.fail('Must not call GitHub for invalid input')
        for repo, sha in [('owner/repo/extra', 'a' * 40), ('owner/repo', 'main')]:
            with self.assertRaises(ValueError):
                prepare(repo, sha, api)
