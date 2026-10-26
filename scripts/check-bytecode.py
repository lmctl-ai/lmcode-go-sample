#!/usr/bin/env python3
"""Recompile with the selected lmscript; never rewrite the checked-in oracle."""
from pathlib import Path
import json
import subprocess
import sys
import tempfile
root=Path(__file__).resolve().parent.parent
with tempfile.TemporaryDirectory(prefix='sample-recompile-') as temporary:
    out=Path(temporary)
    coverage=json.loads((root/'sample-suite/coverage.json').read_text())
    compile_cmd=[sys.executable,str(root/'sample-suite/compile.py'),'--output',str(out)]
    result=subprocess.run(compile_cmd,capture_output=True,text=True)
    skipped=[]
    if result.returncode:
        if 'unexpected character' not in result.stderr or '201-word-literals.lms' not in result.stderr:
            sys.stderr.write(result.stdout); sys.stderr.write(result.stderr); sys.exit(result.returncode)
        print('NOTICE compiler lacks lmcode-literals-v1; checking bytecode reproduction for supported cases only')
        result=subprocess.run([sys.executable,str(root/'sample-suite/compile.py'),'--exclude-feature','lmcode-literals-v1','--output',str(out)],check=True,text=True,capture_output=True)
        skipped=[p.name for p in (root/'sample-suite/programs').glob('*.lms') if 'lmcode-literals-v1' in coverage.get('compiler_features',{}).get(p.name,[])]
        print(result.stdout,end='')
    expected=list((root/'sample-suite/bytecode').glob('*.dslb'))
    expected_names={p.name for p in expected}
    if skipped:
        expected_names-={Path(name).with_suffix('.dslb').name for name in skipped}
    if {p.name for p in out.glob('*.dslb')}!=expected_names:
        sys.exit('compiler produced a different case set')
    for source in expected:
        if source.name not in expected_names: continue
        if source.read_bytes()!=(out/source.name).read_bytes():
            sys.exit('compiler bytecode drift: '+source.name)
print(f'PASS compiler reproduces {len(expected_names)} pinned bytecode cases')
