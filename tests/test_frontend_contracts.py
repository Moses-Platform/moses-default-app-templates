#!/usr/bin/env python3
import sys
import unittest
sys.dont_write_bytecode = True
from frontend_contracts import compiler_is_wired, docker_uses_npm_ci


class FrontendContracts(unittest.TestCase):
    def test_real_run_install(self):
        self.assertTrue(docker_uses_npm_ci('RUN if [ -n "$REGISTRY" ]; then npm config set registry "$REGISTRY"; fi \\\n && npm ci\n'))
        self.assertTrue(docker_uses_npm_ci('RUN npm ci\n'))

    def test_echo_is_not_install(self):
        self.assertFalse(docker_uses_npm_ci('RUN echo "npm ci "\n'))
        self.assertFalse(docker_uses_npm_ci('RUN echo "hello && npm ci "\n'))

    def test_comment_and_non_run_are_not_install(self):
        self.assertFalse(docker_uses_npm_ci('# RUN npm ci\nRUN echo ok\n'))
        self.assertFalse(docker_uses_npm_ci('LABEL note="npm ci "\n'))

    def test_reject_non_reproducible_install(self):
        self.assertFalse(docker_uses_npm_ci('RUN npm ci && npm install\n'))
        self.assertFalse(docker_uses_npm_ci('RUN npm ci; if true; then npm install; fi\n'))

    def test_active_compiler(self):
        self.assertTrue(compiler_is_wired('plugins: [react(), babel({ presets: [reactCompilerPreset()] })]'))

    def test_commented_compiler(self):
        for source in ['// babel({ presets: [reactCompilerPreset()] })\n', '/* babel({ presets: [reactCompilerPreset()] }) */']:
            self.assertFalse(compiler_is_wired(source))


if __name__ == '__main__':
    unittest.main()
