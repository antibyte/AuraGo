import type { IExecuteFunctions, IHookFunctions, ILoadOptionsFunctions } from 'n8n-workflow';
import { NodeApiError } from 'n8n-workflow';
import * as crypto from 'crypto';

/**
 * Make an authenticated API request to AuraGo.
 */
export async function auragoApiRequest(
	this: IExecuteFunctions | ILoadOptionsFunctions | IHookFunctions,
	method: string,
	endpoint: string,
	body: object = {},
	query: object = {},
): Promise<any> {
	const credentials = await this.getCredentials('auraGoApi');
	
	const options = {
		method,
		body,
		qs: query,
		uri: `${credentials.baseUrl}${endpoint}`,
		json: true,
		rejectUnauthorized: !credentials.ignoreSslIssues,
		headers: {
			'Content-Type': 'application/json',
		},
	};

	if (Object.keys(body).length === 0) {
		delete (options as any).body;
	}

	try {
		return await (this.helpers.requestWithAuthentication as any).call(this, 'auraGoApi', options);
	} catch (error) {
		throw new NodeApiError(this.getNode(), error);
	}
}

/**
 * Make a request to get available tools for the load options method.
 */
export async function getTools(this: ILoadOptionsFunctions): Promise<Array<{ name: string; value: string }>> {
	try {
		const response = await auragoApiRequest.call(this, 'GET', '/api/n8n/tools');
		return response.tools.map((tool: { name: string; description: string }) => ({
			name: `${tool.name} - ${tool.description.slice(0, 50)}...`,
			value: tool.name,
		}));
	} catch {
		return [];
	}
}

/**
 * Verify webhook signature from AuraGo.
 */
export function verifyWebhookSignature(
	body: Record<string, unknown>,
	signature: string,
	secret: string,
): boolean {
	const hmac = crypto.createHmac('sha256', secret);
	const jsonData = JSON.stringify(body.data);
	hmac.update(jsonData);
	const computed = hmac.digest('hex');
	
	try {
		return crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(computed));
	} catch {
		return false;
	}
}

/**
 * Convert session messages to conversation format.
 */
export function formatConversationHistory(messages: Array<{ role: string; content: string }>): string {
	return messages.map((msg) => `${msg.role}: ${msg.content}`).join('\n');
}

/**
 * Parse tool calls from agent response.
 */
export function parseToolCalls(toolCalls: Array<{ name: string; arguments: object; result?: string }>): object[] {
	return toolCalls.map((tc) => ({
		tool: tc.name,
		parameters: tc.arguments,
		result: tc.result || 'No result available',
	}));
}
