"""Original AuraGo low-poly collection: exactly 220 authored designs, MIT."""
VERSION = '1.0.0'
PACK_ID = 'aurago-low-poly'
GROUPS = {
    'road': 'compact sedan sports rally suv pickup van minibus bus box-truck tractor-truck trailer tanker tipper fire-engine ambulance police forklift',
    'aircraft': 'prop-plane biplane seaplane fighter-jet airliner cargo-plane helicopter quadcopter',
    'space': 'scout interceptor heavy-fighter shuttle freighter lander planet-earth planet-desert planet-ice planet-lava planet-gas moon asteroid-round asteroid-split asteroid-spire asteroid-ice asteroid-metal station-hub station-habitat station-hangar satellite-comms satellite-science',
    'architecture': 'house townhouse shop office apartment warehouse hangar garage workshop barn water-tower outpost floor ceiling wall wall-window wall-door corner-outer corner-inner pillar beam door-hinged door-sliding window stairs ramp railing railing-corner balcony roof-flat roof-slope roof-ridge roof-gable corridor corridor-corner corridor-tee corridor-cross room bulkhead airlock ladder lift',
    'landscape': 'road-straight road-curve road-tee road-cross road-roundabout road-end road-bridge road-ramp sidewalk sidewalk-corner path path-curve river river-curve river-tee lake ground hill cliff cliff-corner slope rock-arch cave island mountain dune crag landing-pad',
    'vegetation': 'oak birch pine spruce palm acacia baobab alien-tree bush-round bush-tall hedge fern alien-bush grass-short grass-tall flowers reeds cactus-column cactus-branch cactus-paddle stump log',
    'props': 'crate-wood crate-metal barrel-wood barrel-metal pallet supply-case container fuel-tank street-lamp bench bin hydrant cone barrier sign fence table chair sofa bed locker shelf computer-desk kitchen generator pipe pipe-elbow valve console antenna chest keycard crystal checkpoint jump-pad teleporter',
    'humans': 'civilian-a civilian-b mechanic-a mechanic-b pilot-a pilot-b explorer-a explorer-b security-a security-b trooper-a trooper-b',
    'animals': 'dog wolf cat fox deer boar bear horse cow sheep chicken bird',
    'fps': 'pistol smg rifle shotgun sniper energy-pistol plasma-rifle heavy-blaster arms-modern arms-scifi knife grenade medkit ammo-box energy-cell armor-plate reflex-sight scope suppressor shield-generator',
}
EXPECTED_COUNTS = dict(zip(GROUPS, (18, 8, 22, 42, 28, 22, 36, 12, 12, 20)))
HUMAN_ACTIONS = '''idle walk run sprint walk_back strafe_left strafe_right turn_left turn_right crouch_idle crouch_walk jump_start jump_loop jump_land climb fall sit_down sit_idle stand_up pick_up carry_idle carry_walk push interact wave cheer unarmed_idle punch rifle_idle rifle_walk rifle_fire rifle_reload hit death'''.split()
ANIMAL_ACTIONS = 'idle walk run turn_left turn_right eat rest_down rest_idle rest_up hit death'.split()
FPS_ACTIONS = 'draw holster idle walk sprint aim_in aim_idle aim_out fire reload reload_empty'.split()
CATEGORY_NAMES = {
    'road': 'Road vehicles', 'aircraft': 'Aircraft', 'space': 'Space',
    'architecture': 'Buildings and interiors', 'landscape': 'Landscape and roads',
    'vegetation': 'Vegetation', 'props': 'Props and game objects',
    'humans': 'People', 'animals': 'Animals', 'fps': 'First-person equipment',
}
PALETTE = {
    'ivory': '#e4d5b6', 'white': '#ecede5', 'ink': '#273643', 'rubber': '#263035',
    'steel': '#788e9b', 'silver': '#b4c4c9', 'glass': '#284c5c', 'window': '#75b5c7',
    'blue': '#4283a8', 'red': '#c65b4e', 'yellow': '#e0b459', 'teal': '#4c9c96',
    'orange': '#d28a50', 'purple': '#80799e', 'green': '#608966', 'navy': '#344c6a',
    'leaf': '#699b55', 'leaf_light': '#96b96a', 'leaf_dark': '#3b6550',
    'bark': '#78604c', 'wood': '#af8558', 'sand': '#cbb07a', 'soil': '#82715c',
    'stone': '#8e9591', 'road': '#53616b', 'water': '#4f9daa', 'snow': '#c9dedd',
    'skin': '#c89870', 'skin_dark': '#885d49', 'hair': '#4c3e36', 'cyan': '#6cd9d9',
    'pink': '#cf8094', 'cream': '#d9bd8d',
}


def entries():
    result = []
    for category, words in GROUPS.items():
        names = words.split()
        assert len(names) == EXPECTED_COUNTS[category], (category, len(names))
        for index, name in enumerate(names):
            asset_id = category + '-' + name
            result.append(dict(id=asset_id, name=name.replace('-', ' ').title(),
                               category=category, design=name, index=index,
                               description=CATEGORY_NAMES[category] + ': ' + name.replace('-', ' ') + '.',
                               tags=[category, *name.split('-'), 'low-poly', '3d']))
    assert len(result) == 220 and len({x['id'] for x in result}) == 220
    assert len(HUMAN_ACTIONS) == 34
    return result


if __name__ == '__main__':
    print('Verified', len(entries()), 'distinct designs in', len(GROUPS), 'categories.')
