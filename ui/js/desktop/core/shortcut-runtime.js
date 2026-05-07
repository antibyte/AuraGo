    function shortcutKeyMatches(eventKey, eventCode, wanted) {
        if (eventKey === wanted || eventCode === wanted || eventCode === ('key' + wanted)) return true;
        const codeAliases = {
            equal: ['=', '+'],
            numpadadd: ['=', '+'],
            minus: ['-'],
            numpadsubtract: ['-'],
            digit0: ['0'],
            numpad0: ['0']
        };
        if ((codeAliases[eventCode] || []).includes(wanted)) return true;
        const keyAliases = { '=': ['+'], '+': ['='] };
        return (keyAliases[wanted] || []).includes(eventKey);
    }
