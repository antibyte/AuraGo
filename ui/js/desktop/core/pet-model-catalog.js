    // 3D model pets: the fixed allowlist of pet IDs and their asset folders under /3d/pets/<folder>/.
    // The pet runtime, the 3D renderer and the picker posters resolve folders only through
    // petModelAssetFolder (exposed as PetRuntime.modelAssetFolder); an unknown or imported ID never
    // resolves to a model URL. Keep the entries aligned with assets/desktop-pets/catalog.json and
    // service_pets.go.
    const petModelFolders = {
        'aurago-punk-3d': 'punk', 'aurago-zombie-3d': 'zombert',
        'aurago-psycho-3d': 'green', 'aurago-friend-3d': 'hoodie',
        'aurago-thinker-3d': 'professor', 'aurago-neutral-3d': 'silver',
        'aurago-trex-3d': 'rexi', 'aurago-tentacle-3d': 'purple-tentacle',
        'openpets-default-3d': 'openpets-default',
        'snoopy-3d': 'snoopy',
        'clippit-3d': 'clippit',
        'tux-3d': 'tux',
        'wall-e-3d': 'wall-e',
        'dobby-3d': 'dobby',
        'aurago-servant-3d': 'butler',
        'aurago-professional-3d': 'manager',
        'aurago-mistress-3d': 'commander',
        'aurago-evil-3d': 'vampire',
        'aurago-secretary-3d': 'assistant',
        'aurago-mcp-3d': 'mcp',
        'aurago-terminator-3d': 'terminator',
        'aurago-slime-3d': 'glibbi',
        'aurago-spider-3d': 'webbi',
        'aurago-alien-3d': 'zorbit',
        'aurago-indiana-jones-3d': 'indiana-jones',
        'aurago-manga-girl-3d': 'hana',
        'aurago-chick-3d': 'pippin',
        'aurago-peacock-3d': 'pavlo',
        'aurago-puppy-3d': 'buddy',
        'aurago-cat-3d': 'minka',
        'aurago-crocodile-3d': 'kroko'
    };

    function petModelAssetFolder(id) {
        return Object.hasOwn(petModelFolders, id) ? petModelFolders[id] : '';
    }
