import type { ICredentialTestRequest, ICredentialType, INodeProperties, IAuthenticateGeneric } from 'n8n-workflow';

export class AuraGoApi implements ICredentialType {
  name = 'auraGoApi';
  displayName = 'AuraGo API';
  documentationUrl = 'https://github.com/antibyte/aurago/blob/main/n8n-nodes-aurago/README.md';

  properties: INodeProperties[] = [
    {
      displayName: 'Base URL',
      name: 'baseUrl',
      type: 'string',
      default: 'http://localhost:8088',
      required: true,
      description: 'The base URL of your AuraGo instance',
      placeholder: 'http://localhost:8088',
    },
    {
      displayName: 'API Token',
      name: 'apiToken',
      type: 'string',
      typeOptions: { password: true },
      default: '',
      required: true,
      description: 'Create a token in AuraGo Config UI → n8n Integration',
    },
    {
      displayName: 'Ignore SSL Issues',
      name: 'ignoreSslIssues',
      type: 'boolean',
      default: false,
      description: 'Whether to ignore SSL certificate validation errors (useful for self-signed certs)',
    },
  ];

  authenticate: IAuthenticateGeneric = {
    type: 'generic',
    properties: {
      headers: {
        Authorization: '=Bearer {{$credentials.apiToken}}',
      },
    },
  };

  test: ICredentialTestRequest = {
    request: {
      baseURL: '={{ $credentials.baseUrl }}',
      url: '/api/n8n/status',
      skipSslCertificateValidation: '={{ $credentials.ignoreSslIssues }}',
    },
  };
}
