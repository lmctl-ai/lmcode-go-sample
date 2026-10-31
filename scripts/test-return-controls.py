#!/usr/bin/env python3
"""Validate compiled host faults against canonical payloads, never local recordings."""
import json
import os
from pathlib import Path
import runpy
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
SUITE = ROOT / 'sample-suite'
validate = runpy.run_path(str(SUITE / 'return_protocol.py'))['validate']
cases = {c['name']: c for c in json.loads((SUITE / 'manifest.json').read_text())['cases']}
source = (ROOT / 'main.go').read_text()
variants = [
    ('baseline', source, None),
    ('missing-frame', source.replace('if err := lmcode.WriteResultFrame(os.Stdout, &sink); err != nil {', 'if err := error(nil); err != nil {'), '001-literals'),
    ('reset-between-entries', source.replace('callEntry := func(index int, argument lmcode.Value) error {', 'callEntry := func(index int, argument lmcode.Value) error {\n\t\tsink.Reset()'), '364-return-entry-lifetime'),
]
for name, changed, target in variants:
    if target and changed == source:
        raise AssertionError('mutation anchor missing: ' + name)
    with tempfile.TemporaryDirectory(prefix='go-return-control-') as temp:
        root = Path(temp)
        for file in ['go.mod', 'go.sum', 'main.go', 'host.go']:
            shutil.copy2(ROOT / file, root / file)
        (root / 'sample-suite').mkdir()
        shutil.copy2(SUITE / 'sample.dsl.json', root / 'sample-suite/sample.dsl.json')
        (root / 'main.go').write_text(changed)
        binary = root / ('host.exe' if os.name == 'nt' else 'host')
        subprocess.run(['go', 'build', '-mod=readonly', '-trimpath', '-o', str(binary), '.'], cwd=root, env=dict(os.environ, CGO_ENABLED='0', GOWORK='off'), check=True, capture_output=True)
        for case_name in ([target] if target else ['001-literals', '364-return-entry-lifetime']):
            case = cases[case_name]
            result = subprocess.run([str(binary), '--dsl', str(SUITE / case['bytecode'])], cwd=root, env=dict(os.environ, SAMPLE_TMP=temp), capture_output=True)
            assert result.returncode == 0 and result.stderr == b'', (name, result.returncode, result.stderr)
            try:
                validate(result.stdout, result.returncode, case.get('expected_return'))
            except ValueError:
                if not target:
                    raise
                print('CAUGHT ' + name)
            else:
                if target:
                    raise AssertionError('mutation survived: ' + name)
                assert result.stdout == (SUITE / 'expected' / (case_name + '.out')).read_bytes()
        if not target:
            print('PASS baseline')
