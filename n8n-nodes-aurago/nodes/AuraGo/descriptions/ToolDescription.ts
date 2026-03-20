import type { INodeProperties } from 'n8n-workflow';

export const toolOperations: INodeProperties[] = [
	{
		displayName: 'Operation',
		name: 'operation',
		type: 'options',
		noDataExpression: true,
		displayOptions: {
			show: {
				resource: ['tool'],
			},
		},
		options: [
			{
				name: 'Execute',
				value: 'execute',
				description: 'Execute a specific tool directly',
				action: 'Execute a specific tool',
			},
			{
				name: 'List Available',
				value: 'list',
				description: 'Get a list of available tools',
				action: 'List available tools',
			},
		],
		default: 'execute',
	},
];

export const toolFields: INodeProperties[] = [
	// Execute Tool
	{
		displayName: 'Tool Name',
		name: 'toolName',
		type: 'options',
		typeOptions: {
			loadOptionsMethod: 'getTools',
		},
		default: '',
		required: true,
		displayOptions: {
			show: {
				resource: ['tool'],
				operation: ['execute'],
			},
		},
		description: 'The tool to execute',
	},
	{
		displayName: 'Parameters',
		name: 'parameters',
		type: 'json',
		default: '{}',
		typeOptions: {
			rows: 6,
		},
		displayOptions: {
			show: {
				resource: ['tool'],
				operation: ['execute'],
			},
		},
		description: 'Tool parameters as JSON object',
	},
	{
		displayName: 'Timeout',
		name: 'timeout',
		type: 'number',
		default: 60,
		displayOptions: {
			show: {
				resource: ['tool'],
				operation: ['execute'],
			},
		},
		description: 'Maximum execution time in seconds',
	},
];
