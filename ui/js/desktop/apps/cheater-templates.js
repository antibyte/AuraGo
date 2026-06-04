(function () {
    'use strict';

    const TEMPLATES = [
        { id: 'empty', icon: '📄', nameKey: 'cheater.template.empty', content: '# {{title}}\n\n' },
        { id: 'deployment', icon: '🚀', nameKey: 'cheater.template.deployment',
          content: '# {{title}}\n\n## Pre-Flight\n- [ ] Backup erstellt\n- [ ] Wartungsfenster kommuniziert\n- [ ] Rollback-Plan dokumentiert\n\n## Steps\n1. \n2. \n3. \n\n## Rollback\n- \n' },
        { id: 'debug', icon: '🔍', nameKey: 'cheater.template.debug',
          content: '# {{title}}\n\n## Symptom\n- \n\n## Hypothese\n- \n\n## Reproduktion\n1. \n2. \n3. \n\n## Fix\n- \n\n## Verifikation\n- [ ] Fix getestet\n- [ ] Regression-Tests grün\n' },
        { id: 'routine', icon: '☀️', nameKey: 'cheater.template.routine',
          content: '# {{title}}\n\n## Morgens\n- [ ] \n- [ ] \n\n## Mittags\n- [ ] \n\n## Abends\n- [ ] \n' },
        { id: 'api', icon: '🔌', nameKey: 'cheater.template.api',
          content: '# {{title}}\n\n## GET /endpoint\n```bash\ncurl -sS https://api.example.com/endpoint\n```\n\n## POST /endpoint\n```bash\ncurl -sS -X POST -H "Content-Type: application/json" -d \'{}\' https://api.example.com/endpoint\n```\n' },
        { id: 'backup', icon: '🛡️', nameKey: 'cheater.template.backup',
          content: '# {{title}}\n\n## Was\n- \n\n## Wann\n- Täglich: \n- Wöchentlich: \n- Monatlich: \n\n## Wohin\n- \n\n## Restore-Test\n- [ ] Quartalsweise getestet\n' }
    ];

    function listTemplates(t) {
        return TEMPLATES.map(tpl => ({
            id: tpl.id,
            icon: tpl.icon,
            name: t(tpl.nameKey, tpl.id),
            content: tpl.content
        }));
    }

    function templateById(id) {
        return TEMPLATES.find(tpl => tpl.id === id) || TEMPLATES[0];
    }

    window.CheaterTemplates = window.CheaterTemplates || {};
    window.CheaterTemplates.list = listTemplates;
    window.CheaterTemplates.byId = templateById;
})();
