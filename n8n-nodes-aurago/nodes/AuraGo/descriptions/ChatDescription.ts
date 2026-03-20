import type { INodeProperties } from 'n8n-workflow';

export const chatOperations: INodeProperties[] = [
	{
		displayName: 'Operation',
		name: 'operation',
		type: 'options',
		noDataExpression: true,
		displayOptions: {
			show: {
				resource: ['chat'],
			},
		},
		options: [
			{
				name: 'Send Message',
				value: 'sendMessage',
				description: 'Send a message to the AuraGo agent',
				action: 'Send a message to the AuraGo agent',
			},
			{
				name: 'Continue Session',
				value: 'continueSession',
				description: 'Continue an existing conversation session',
				action: 'Continue an existing conversation session',
			},
		],
		default: 'sendMessage',
	},
];

export const chatFields: INodeProperties[] = [
	// Send Message
	{
		displayName: 'Message',
		name: 'message',
		type: 'string',
		typeOptions: {
			rows: 4,
		},
		default: '',
		required: true,
		displayOptions: {
			show: {
				resource: ['chat'],
				operation: ['sendMessage', 'continueSession'],
			},
		},
		description: 'The message to send to the agent',
	},
	{
		displayName: 'Session ID',
		name: 'sessionId',
		type: 'string',
		default: '',
		displayOptions: {
			show: {
				resource: ['chat'],
				operation: ['continueSession'],
			},
		},
		description: 'The session ID to continue (from a previous message)',
	},
	{
		displayName: 'Context Window',
		name: 'contextWindow',
		type: 'number',
		default: 10,
		displayOptions: {
			show: {
				resource: ['chat'],
				operation: ['sendMessage', 'continueSession'],
			},
		},
		description: 'Number of previous messages to include in context',
	},
	{
		displayName: 'System Prompt',
		name: 'systemPrompt',
		type: 'string',
		typeOptions: {
			rows: 4,
		},
		default: '',
		displayOptions: {
			show: {
				resource: ['chat'],
				operation: ['sendMessage'],
			},
		},
		description: 'Optional system prompt to override the default agent behavior',
	},
	{
		displayName: 'Restrict Tools',
		name: 'restrictTools',
		type: 'boolean',
		default: false,
		displayOptions: {
			show: {
				resource: ['chat'],
				operation: ['sendMessage', 'continueSession'],
			},
		},
		description: 'Whether to restrict which tools the agent can use',
	},
	{
		displayName: 'Allowed Tools',
		name: 'allowedTools',
		type: 'multiOptions',
		typeOptions: {
			loadOptionsMethod: 'getTools',
		},
		default: [],
		displayOptions: {
			show: {
				resource: ['chat'],
				operation: ['sendMessage', 'continueSession'],
				restrictTools: [true],
			},
		},
		description: 'Select which tools the agent is allowed to use',
	},
];
