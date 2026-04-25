module.exports = {
	root: true,
	env: {
		browser: true,
		es6: true,
		node: true,
	},
	parser: '@typescript-eslint/parser',
	parserOptions: {
		project: './tsconfig.json',
		sourceType: 'module',
		ecmaVersion: 2020,
		extraFileExtensions: ['.json'],
	},
	plugins: ['@typescript-eslint', 'eslint-plugin-n8n-nodes-base'],
	extends: [
		'eslint:recommended',
		'plugin:@typescript-eslint/recommended',
		'plugin:prettier/recommended',
		'plugin:n8n-nodes-base/community',
		'plugin:n8n-nodes-base/credentials',
		'plugin:n8n-nodes-base/nodes',
	],
		rules: {
		'@typescript-eslint/no-explicit-any': 'off',
		'@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
		'n8n-nodes-base/cred-class-field-authenticate-type-assertion': 'off',
		'n8n-nodes-base/cred-class-field-documentation-url-miscased': 'off',
		'n8n-nodes-base/node-param-operation-option-action-miscased': 'off',
		'n8n-nodes-base/node-param-type-options-max-value-present': 'off',
	},
};
