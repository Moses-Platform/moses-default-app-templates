#!/usr/bin/env python3
"""Small static gates, supplemented by each frontend's real compiler build test."""
import json
import re
import shlex
import sys
from pathlib import Path


def docker_uses_npm_ci(source):
    # Join Docker continuations, then inspect only executable RUN instructions.
    source = re.sub(r'\\\n', ' ', source)
    commands = []
    for line in source.splitlines():
        match = re.match(r'^\s*RUN\s+(.+)$', line, re.I)
        if not match:
            continue
        body = match[1]
        if body.startswith('['):
            commands.append(json.loads(body))
            continue
        lexer = shlex.shlex(body, posix=True, punctuation_chars=';&|()')
        lexer.whitespace_split = True
        command = []
        for token in lexer:
            if token in ('&&', '||', ';', '|', '(', ')', '&'):
                if command:
                    commands.append(command)
                command = []
            else:
                command.append(token)
        if command:
            commands.append(command)
    for command in commands:
        # Shell control words and environment assignments precede the command.
        while command and (command[0] in ('then', 'else', 'do', '!')
                           or re.match(r'^[A-Za-z_][A-Za-z0-9_]*=', command[0])):
            command.pop(0)
    # This check guards against regressing to npm install; it is not a shell parser.
    return (any(c[:2] == ['npm', 'ci'] for c in commands)
            and not any(c[:2] in (['npm', 'install'], ['npm', 'i'], ['npm', 'add']) for c in commands))


def compiler_is_wired(source):
    # Preserve string literals while removing both JS comment forms. The actual
    # build test also rejects dead code, unused plugins and compiler bailouts.
    tokens = r'''"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|`(?:\\.|[^`\\])*`|//[^\n]*|/\*[\s\S]*?\*/'''
    source = re.sub(tokens, lambda m: '' if m[0].startswith(('//', '/*')) else m[0], source)
    return (bool(re.search(r'babel\(\{\s*presets:\s*\[reactCompilerPreset\(\)\]\s*\}\)', source))
            and not re.search(r'react\(\{\s*babel', source))


if __name__ == '__main__':
    check = {'docker': docker_uses_npm_ci, 'compiler': compiler_is_wired}[sys.argv[1]]
    sys.exit(0 if check(Path(sys.argv[2]).read_text()) else 1)
