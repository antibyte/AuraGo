    function shortcutKeyMatches(eventKey, eventCode, wanted) {
        if (eventKey === wanted || eventCode === wanted || eventCode === ('Key' + wanted)) return true;
        const code = eventCode ? eventCode.toLowerCase() : '';
        const codeAliases = {
            equal: ['=', '+'],
            numpadadd: ['=', '+'],
            minus: ['-'],
            numpadsubtract: ['-'],
            digit0: ['0'],
            numpad0: ['0']
        };
        if ((codeAliases[code] || []).includes(wanted)) return true;
        const keyAliases = { '=': ['+'], '+': ['='] };
        return (keyAliases[wanted] || []).includes(eventKey);
    }
