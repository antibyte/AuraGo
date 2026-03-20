import type {
	IExecuteFunctions,
	INodeExecutionData,
	INodeType,
	INodeTypeDescription,
	ILoadOptionsFunctions,
} from 'n8n-workflow';

import {
	chatOperations,
	chatFields,
	toolOperations,
	toolFields,
	memoryOperations,
	memoryFields,
	missionOperations,
	missionFields,
} from './descriptions';

import { auragoApiRequest, getTools } from './GenericFunctions';

export class AuraGo implements INodeType {
	description: INodeTypeDescription = {
		displayName: 'AuraGo',
		name: 'auraGo',
		icon: 'file:aurago.svg',
		group: ['transform'],
		version: 1,
		subtitle: '={{ $parameter["operation"] }}',
		description: 'Interact with AuraGo AI Agent',
		defaults: {
			name: 'AuraGo',
		},
		inputs: ['main'],
		outputs: ['main'],
		credentials: [
			{
				name: 'auraGoApi',
				required: true,
			},
		],
		properties: [
			{
				displayName: 'Resource',
				name: 'resource',
				type: 'options',
				noDataExpression: true,
				options: [
					{
						name: 'Chat',
						value: 'chat',
						description: 'Chat with the AI agent',
					},
					{
						name: 'Tool',
						value: 'tool',
						description: 'Execute tools directly',
					},
					{
						name: 'Memory',
						value: 'memory',
						description: 'Search or store memories',
					},
					{
						name: 'Mission',
						value: 'mission',
						description: 'Create and manage missions',
					},
				],
				default: 'chat',
			},
			...chatOperations,
			...chatFields,
			...toolOperations,
			...toolFields,
			...memoryOperations,
			...memoryFields,
			...missionOperations,
			...missionFields,
		],
	};

	methods = {
		loadOptions: {
			getTools,
		},
	};

	async execute(this: IExecuteFunctions): Promise<INodeExecutionData[][]> {
		const items = this.getInputData();
		const returnData: INodeExecutionData[] = [];
		const resource = this.getNodeParameter('resource', 0) as string;
		const operation = this.getNodeParameter('operation', 0) as string;

		for (let i = 0; i < items.length; i++) {
			try {
				let response;

				switch (resource) {
					case 'chat':
						response = await executeChatOperation.call(this, i, operation);
						break;
					case 'tool':
						response = await executeToolOperation.call(this, i, operation);
						break;
					case 'memory':
						response = await executeMemoryOperation.call(this, i, operation);
						break;
					case 'mission':
						response = await executeMissionOperation.call(this, i, operation);
						break;
					default:
						throw new Error(`Unknown resource: ${resource}`);
				}

				returnData.push({
					json: response,
					pairedItem: { item: i },
				});
				} catch (error: any) {
				if (this.continueOnFail()) {
					returnData.push({
						json: { error: error?.message || 'Unknown error' },
						pairedItem: { item: i },
					});
				} else {
					throw error;
				}
			}
		}

		return [returnData];
	}
}

async function executeChatOperation(
	this: IExecuteFunctions,
	index: number,
	operation: string,
): Promise<any> {
	const message = this.getNodeParameter('message', index) as string;
	const contextWindow = this.getNodeParameter('contextWindow', index) as number;

	const body: Record<string, any> = {
		message,
		context_window: contextWindow,
	};

	if (operation === 'continueSession') {
		const sessionId = this.getNodeParameter('sessionId', index) as string;
		body.session_id = sessionId;
	}

	const systemPrompt = this.getNodeParameter('systemPrompt', index, '') as string;
	if (systemPrompt) {
		body.system_prompt = systemPrompt;
	}

	const restrictTools = this.getNodeParameter('restrictTools', index, false) as boolean;
	if (restrictTools) {
		const allowedTools = this.getNodeParameter('allowedTools', index, []) as string[];
		if (allowedTools.length > 0) {
			body.tools = allowedTools;
		}
	}

	return await auragoApiRequest.call(this, 'POST', '/api/n8n/chat', body);
}

async function executeToolOperation(
	this: IExecuteFunctions,
	index: number,
	operation: string,
): Promise<any> {
	if (operation === 'list') {
		return await auragoApiRequest.call(this, 'GET', '/api/n8n/tools');
	}

	const toolName = this.getNodeParameter('toolName', index) as string;
	const parametersJson = this.getNodeParameter('parameters', index, '{}') as string;
	const timeout = this.getNodeParameter('timeout', index, 60) as number;
	const async = this.getNodeParameter('async', index, false) as boolean;

	let parameters = {};
	try {
		parameters = JSON.parse(parametersJson);
	} catch {
		throw new Error('Invalid JSON in Parameters field');
	}

	const body = {
		parameters,
		timeout,
		async,
	};

	return await auragoApiRequest.call(this, 'POST', `/api/n8n/tools/${toolName}`, body);
}

async function executeMemoryOperation(
	this: IExecuteFunctions,
	index: number,
	operation: string,
): Promise<any> {
	if (operation === 'search') {
		const query = this.getNodeParameter('query', index) as string;
		const memoryType = this.getNodeParameter('memoryType', index, 'all') as string;
		const limit = this.getNodeParameter('limit', index, 10) as number;

		const body: Record<string, any> = {
			query,
			limit,
		};

		if (memoryType !== 'all') {
			body.type = memoryType;
		}

		return await auragoApiRequest.call(this, 'POST', '/api/n8n/memory/search', body);
	}

	// store operation
	const content = this.getNodeParameter('content', index) as string;
	const storageType = this.getNodeParameter('storageType', index) as string;
	const metadataJson = this.getNodeParameter('metadata', index, '{}') as string;

	let metadata = {};
	try {
		metadata = JSON.parse(metadataJson);
	} catch {
		throw new Error('Invalid JSON in Metadata field');
	}

	const body = {
		content,
		type: storageType,
		metadata,
	};

	return await auragoApiRequest.call(this, 'POST', '/api/n8n/memory/store', body);
}

async function executeMissionOperation(
	this: IExecuteFunctions,
	index: number,
	operation: string,
): Promise<any> {
	const missionName = this.getNodeParameter('missionName', index) as string;
	const description = this.getNodeParameter('description', index, '') as string;
	const triggerType = this.getNodeParameter('triggerType', index, 'manual') as string;
	const stepsJson = this.getNodeParameter('steps', index, '[]') as string;

	let steps = [];
	try {
		steps = JSON.parse(stepsJson);
	} catch {
		throw new Error('Invalid JSON in Steps field');
	}

	const body: Record<string, any> = {
		name: missionName,
		description,
		trigger: triggerType,
		steps,
		run_now: operation === 'createAndRun',
	};

	return await auragoApiRequest.call(this, 'POST', '/api/n8n/missions', body);
}
