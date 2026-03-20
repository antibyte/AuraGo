import type {
	IWebhookFunctions,
	INodeType,
	INodeTypeDescription,
	IWebhookResponseData,
	NodeConnectionType,
	IDataObject,
} from 'n8n-workflow';

import { verifyWebhookSignature } from './GenericFunctions';

export class AuraGoTrigger implements INodeType {
	description: INodeTypeDescription = {
		displayName: 'AuraGo Trigger',
		name: 'auraGoTrigger',
		icon: 'file:aurago.svg',
		group: ['trigger'],
		version: 1,
		description: 'Trigger workflows from AuraGo events',
		defaults: {
			name: 'AuraGo Trigger',
		},
		inputs: [],
		outputs: [NodeConnectionType.Main],
		credentials: [
			{
				name: 'auraGoApi',
				required: false,
			},
		],
		webhooks: [
			{
				name: 'default',
				httpMethod: 'POST',
				responseMode: 'onReceived',
				path: 'webhook',
			},
		],
		properties: [
			{
				displayName: 'Events',
				name: 'events',
				type: 'multiOptions',
				options: [
					{
						name: 'Agent Response',
						value: 'agent.response',
						description: 'Triggered when the agent generates a response',
					},
					{
						name: 'Tool Call',
						value: 'agent.tool_call',
						description: 'Triggered when a tool is executed',
					},
					{
						name: 'Error',
						value: 'agent.error',
						description: 'Triggered when an error occurs',
					},
					{
						name: 'Memory Stored',
						value: 'memory.stored',
						description: 'Triggered when new memory is stored',
					},
					{
						name: 'Mission Completed',
						value: 'mission.completed',
						description: 'Triggered when a mission finishes',
					},
				],
				default: ['agent.response'],
				required: true,
				description: 'Which events should trigger this workflow',
			},
			{
				displayName: 'Session Filter',
				name: 'sessionFilter',
				type: 'string',
				default: '',
				placeholder: 'e.g., session_123',
				description: 'Only trigger for a specific session ID (optional)',
			},
			{
				displayName: 'Verify Signature',
				name: 'verifySignature',
				type: 'boolean',
				default: true,
				description: 'Whether to verify the webhook signature using the API token',
			},
		],
	};

	async webhook(this: IWebhookFunctions): Promise<IWebhookResponseData> {
		const body = this.getBodyData() as IDataObject;
		const events = this.getNodeParameter('events') as string[];
		const sessionFilter = this.getNodeParameter('sessionFilter', '') as string;
		const verifySignature = this.getNodeParameter('verifySignature', true) as boolean;

		// Filter by event type
		const eventType = body.event as string;
		if (!events.includes(eventType)) {
			return {
				workflowData: [[]],
			};
		}

		// Filter by session ID if specified
		if (sessionFilter) {
			const sessionId = body.session_id as string;
			if (sessionId !== sessionFilter) {
				return {
					workflowData: [[]],
				};
			}
		}

		// Verify webhook signature if enabled
		if (verifySignature) {
			const credentials = await this.getCredentials('auraGoApi');
			const signature = body.signature as string;
			
			if (!signature) {
				return {
					workflowData: [[]],
					webhookResponse: {
						status: 401,
						body: 'Missing signature',
					},
				};
			}

			if (!verifyWebhookSignature(body as Record<string, unknown>, signature, credentials.apiToken as string)) {
				return {
					workflowData: [[]],
					webhookResponse: {
						status: 401,
						body: 'Invalid signature',
					},
				};
			}
		}

		// Return the event data
		return {
			workflowData: [
				[
					{
						json: body,
					},
				],
			],
		};
	}
}
