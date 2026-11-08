"""Check retained reconstruction coverage and rigid-part inputs without Blender (MIT)."""
import gzip
import hashlib
import json
import math
from pathlib import Path

HERE = Path(__file__).resolve().parent
CARS = {"vektor", "kobalt", "lumen", "tundra", "softeis", "maehkoenig", "couch", "bertha"}
PROPS = {"grandstand", "gnome_statue", "hotdog_stand", "camper_van", "chalet", "cablecar_pylon",
         "water_tower", "windmill_pump", "lighthouse", "tugboat", "ufo", "rover", "watermill",
         "hunting_hide", "cooling_tower", "chem_tank", "research_station", "snow_cat", "food_truck", "metro_entrance"}


def check():
    sources = json.loads((HERE / "sources.json").read_text(encoding="utf-8"))["assets"]
    expected = {"car-" + car for car in CARS} | PROPS
    assert len(sources) == 28 and {s["id"] for s in sources} == expected
    groups = {}
    for source in sources:
        for key in ("image_sha256", "model_sha256"):
            assert len(source[key]) == 64 and all(c in "0123456789abcdef" for c in source[key])
        assert source["request_id"] and len(source["request_id"]) == 32, source["id"]
        group = source["group"]
        if group not in groups:
            with gzip.open(HERE / (group + ".json.gz"), "rt", encoding="utf-8") as handle:
                groups[group] = json.load(handle)
        assert source["node"] in {n["name"] for n in groups[group]["nodes"]}, source["id"]
    for group, data in groups.items():
        names = [n["name"] for n in data["nodes"]]
        assert len(names) == len(set(names)), group
        for node in data["nodes"]:
            assert node["parts"], (group, node["name"])
            for geometry in node["parts"].values():
                verts, faces, cols = geometry["verts"], geometry["faces"], geometry["cols"]
                assert verts and faces and len(verts) == len(cols), group
                assert all(len(v) == 3 and all(math.isfinite(c) for c in v) for v in verts), group
                assert all(len(c) == 4 and all(0 <= v <= 1 for v in c) for c in cols), group
                assert all(len(f) >= 3 and all(isinstance(i, int) and 0 <= i < len(verts) for i in f) for f in faces), group
                if "uvs" in geometry:
                    assert len(geometry["uvs"]) == sum(map(len, faces)), (group, "UV loop count")
                    assert all(len(uv) == 2 and all(math.isfinite(v) and -.001 <= v <= 1.001 for v in uv) for uv in geometry["uvs"]), group
        if group.startswith("car-"):
            body = next(n for n in data["nodes"] if n["name"] == "body")
            assert set(body["parts"]) == {"paint", "trim", "head_light", "brake_light"}, group
            assert "uvs" in body["parts"]["paint"], group
            texture = HERE / data["texture"]
            assert texture.parent == HERE and texture.suffix == ".webp", group
            encoded = texture.read_bytes()
            assert encoded[:4] == b"RIFF" and encoded[8:12] == b"WEBP" and len(encoded) <= 350 * 1024, group
            prep = data["preparation"]
            assert prep["closed_wheel_cuts"] == 4 and prep["boundary_edges"] <= prep["source_boundary_edges"], group
            assert data["texture_sha256"] == hashlib.sha256(encoded).hexdigest(), group
            wheel = data["wheels"]
            assert 0.2 < wheel["radius"] < 1.5 and wheel["width"] > 0, group
            assert set(wheel["positions"]) == {"wheel_" + axle + side for axle in ("f", "r") for side in ("l", "r")}
            assert all(abs(p[1] - wheel["radius"]) < 0.00001 for p in wheel["positions"].values()), group
            required = {"maehkoenig": {"gnome"}, "couch": {"lamp", "cushion"}, "softeis": {"cone"}}.get(group[4:], set())
            assert required.issubset(names), group
        if group == "scenery-desert":
            assert "rotor" in names and next(n for n in data["nodes"] if n["name"] == "rotor")["location"][1] > 6
    print("Verified 28 retained reconstructions, eight UV paint atlases, closed cuts and rigid-part inputs")


if __name__ == "__main__":
    check()
