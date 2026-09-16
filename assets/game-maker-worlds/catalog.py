"""Authoritative original motifs; frames and palette variants are not entries."""
VERSION = '1.0.0'
MARITIME = {
 'ships': 'rowboat dinghy cutter sloop schooner brig frigate galleon carrack junk fishing-boat paddle-steamer ocean-steamer ironclad'.split(),
 'submarines': 'diving-capsule research-sub tandem-sub torpedo-sub cargo-sub steampunk-sub'.split(),
 'coast': 'island atoll rocky-island volcano beach-straight beach-corner coast-inlet cliff-straight cliff-corner sea-cave reef seabed'.split(),
 'nature': 'coconut-palm fan-palm bent-palm royal-palm banana breadfruit banyan mangrove hibiscus sea-grape fern beach-grass kelp sea-grass brain-coral branching-coral sea-fan anemone'.split(),
 'harbor': 'beach-hut stilt-house warehouse tavern shipyard watchtower lighthouse naval-quarters floor wall window-wall doorway pitched-roof flat-roof door stairs ramp pier pier-corner pier-cross railing railing-corner mooring-post dock-ladder'.split(),
 'equipment': 'cannon swivel-gun cannonballs torpedo depth-charge harpoon anchor capstan ship-wheel compass spyglass treasure-chest barrel crate rope-coil net buoy lantern flag ship-bell diving-bell boiler pump gear-mechanism'.split(),
 'animals': 'reef-fish schooling-fish tuna ray sea-turtle dolphin octopus seagull reef-shark hammerhead'.split(),
 'people': 'pirate-captain pirate-gunner pirate-swashbuckler pirate-scout naval-officer naval-gunner naval-marine naval-sailor diver-brass diver-light fisherman merchant'.split(),
}
ISO = {
 'terrain': [f'{material}-{shape}' for material in ('grass','soil','sand','rock','snow','metal') for shape in ('flat','block','edge','outer-corner','inner-corner','slope','stairs','pillar')]
 + 'water-deep water-shallow water-shore water-corner water-inlet river-straight river-bend river-fork canal-straight canal-lock waterfall lava-flat lava-edge'.split()
 + [f'{material}-{shape}' for material in ('path','road','cobble') for shape in ('straight','corner','t-junction','cross','end')]
 + 'bridge-wood bridge-stone bridge-metal bridge-ramp'.split(),
 'architecture': [f'{style}-{module}' for style in ('village','city') for module in 'floor ceiling wall window-wall door-wall outer-corner inner-corner pillar beam door window stairs ramp rail rail-corner balcony roof roof-corner ridge gable arch chimney awning fence'.split()]
 + [f'scifi-{module}' for module in 'floor ceiling wall window-wall door-wall corner pillar door stairs ramp rail roof corridor corridor-corner airlock lift'.split()],
 'nature': 'oak birch pine spruce palm acacia willow dead-tree hedge round-bush fern thorn-bush grass flowers reeds moss boulder rock-pile stalagmite crystal mushroom cluster-mushroom stump log'.split(),
 'objects': 'table chair stool sofa bed wardrobe shelf desk cabinet sink oven workbench anvil forge market-stall cartwheel crate barrel chest sack basket jar rope tools sign torch lantern lamp streetlamp brazier generator pipe valve console antenna door-wood door-metal lever pressure-plate coin'.split(),
 'people': 'adventurer ranger traveler scholar knight archer villager blacksmith citizen mechanic medic guard explorer engineer trooper android'.split(),
 'animals': 'dog cat horse cow sheep boar chicken bird'.split(),
 'vehicles': 'handcart wagon car truck hovercraft shuttle rowboat steamship'.split(),
}
EXPECTED_MARITIME = dict(ships=14, submarines=6, coast=12, nature=18, harbor=24, equipment=24, animals=10, people=12)
EXPECTED_ISO = dict(terrain=80, architecture=64, nature=24, objects=40, people=16, animals=8, vehicles=8)

def entries(isometric=False):
    groups = ISO if isometric else MARITIME
    expected = EXPECTED_ISO if isometric else EXPECTED_MARITIME
    assert {key: len(value) for key, value in groups.items()} == expected
    result = []
    for category, names in groups.items():
        for index, design in enumerate(names):
            result.append(dict(id=category+'-'+design, category=category, design=design,
                name=design.replace('-', ' ').title(), index=index,
                description=design.replace('-', ' ').capitalize()+' for '+('isometric worlds' if isometric else 'pirate and steampunk maritime worlds')+'.',
                tags=[category, design, 'isometric' if isometric else 'maritime']))
    assert len({entry['id'] for entry in result}) == len(result)
    return result

if __name__ == '__main__':
    print('Maritime:', len(entries()), 'Isometric:', len(entries(True)))
