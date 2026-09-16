"""Retain original Blender scenes and GLBs used to produce isometric pixels."""
from pathlib import Path
import importlib.util,json,argparse,sys
HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('world_build',HERE/'build.py')
pipeline=importlib.util.module_from_spec(spec);spec.loader.exec_module(pipeline)
import isometric_shapes
pipeline.create=isometric_shapes.create
pipeline.OUT=HERE/'production/isometric-glb'
pipeline.PRODUCTION=HERE/'production/isometric'
pipeline.base.OUT=pipeline.OUT
pipeline.OUT.mkdir(parents=True,exist_ok=True)
records=[]
parser=argparse.ArgumentParser();parser.add_argument('--only')
args=parser.parse_args(sys.argv[sys.argv.index('--')+1:] if '--' in sys.argv else [])
ids=set(args.only.split(',')) if args.only else None
for entry in pipeline.catalog.entries(True):
    if ids and entry['id'] not in ids:continue
    records.append(pipeline.build_one(entry))
    print('ISOMETRIC_SOURCE',entry['id'],flush=True)
previous=pipeline.OUT/'manifest.json'
if ids and previous.exists():records=[a for a in json.loads(previous.read_text())['assets'] if a['id'] not in ids]+records
pipeline.write_json(previous,dict(id='aurago-isometric',version='1.0.0',assets=sorted(records,key=lambda a:a['id'])))
