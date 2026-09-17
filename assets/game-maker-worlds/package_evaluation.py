"""Package an isolated provider-host evaluation; never includes configuration."""
from pathlib import Path
import argparse, hashlib, shutil, tarfile

ROOT=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser()
parser.add_argument('--out',required=True)
parser.add_argument('--binary',required=True)
args=parser.parse_args()
out=Path(args.out).resolve();out.mkdir(parents=True,exist_ok=True)
shutil.copyfile(args.binary,out/'game-maker-eval-linux.test')
sources=[]
for directory in ('runtime','asset_packs'):
    base=ROOT/'internal/gamemaker'/directory
    for path in base.rglob('*'):
        if path.is_file() and 'production' not in path.relative_to(base).parts and path.suffix in ('.js','.json','.png','.webp','.glb','.wav','.txt','.md'):
            sources.append(path)
sources.append(ROOT/'internal/gamemaker/testdata/studio-parent.html')
with tarfile.open(out/'game-maker-eval-assets.tar.gz','w:gz') as archive:
    for source in sources:
        archive.add(source,arcname=source.relative_to(ROOT).as_posix(),recursive=False)
runner='''#!/usr/bin/env bash
set -euo pipefail
bundle="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$bundle"
sha256sum -c SHA256SUMS
tar -xzf game-maker-eval-assets.tar.gz -C "$bundle"
mkdir -p "$bundle/internal/server"
chmod 700 game-maker-eval-linux.test
results="$bundle/results/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$results"
agnes_model="${GAMEMAKER_EVAL_AGNES_MODEL:-agnes-3.0-flash}"
stepfun_model="${GAMEMAKER_EVAL_STEPFUN_MODEL:-step-3.7-flash}"
provider="${GAMEMAKER_EVAL_PROVIDER:-}"
task="${GAMEMAKER_EVAL_TASK:-}"
printf '%s\\n' "World-pack comparison; results: $results" 'Keep Chrome connected to the evaluation parent on port 8896; generation waits for that connection.'
printf '%s\\n' "Expected models: Agnes $agnes_model; StepFun $stepfun_model"
printf '%s\\n' "Filters: provider=${provider:-all}; task=${task:-all}. Private workspaces remain on this server for diagnosis."
exec sudo systemd-run --unit="aurago-worlds-eval-$(date +%s)" --wait --collect --pipe --uid=aurago \\
  --property="WorkingDirectory=$bundle/internal/server" \\
  --property=EnvironmentFile=/etc/aurago/master.key --property=UMask=0077 \\
  --setenv=GAMEMAKER_EVAL_CONFIG=/home/aurago/aurago/config.yaml \\
  --setenv=GAMEMAKER_EVAL_WORLDS=1 --setenv="GAMEMAKER_EVAL_REPORT_DIR=$results" \\
  --setenv="GAMEMAKER_EVAL_AGNES_MODEL=$agnes_model" --setenv="GAMEMAKER_EVAL_STEPFUN_MODEL=$stepfun_model" \\
  --setenv="GAMEMAKER_EVAL_PROVIDER=$provider" --setenv="GAMEMAKER_EVAL_TASK=$task" \\
  --setenv=GAMEMAKER_EVAL_KEEP_WORKSPACE=1 \\
  "$bundle/game-maker-eval-linux.test" -test.run '^TestGameMakerLiveEvaluation$' -test.v -test.timeout 200m
'''
(out/'run-model-comparison.sh').write_text(runner,encoding='utf-8',newline='\n')
names=['game-maker-eval-linux.test','game-maker-eval-assets.tar.gz','run-model-comparison.sh']
(out/'SHA256SUMS').write_text(''.join(hashlib.sha256((out/name).read_bytes()).hexdigest()+'  '+name+'\n' for name in names),encoding='utf-8',newline='\n')
print('Packaged',len(sources),'repository files; no provider configuration or credentials:',out)
