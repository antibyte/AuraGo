import type { INodeProperties } from 'n8n-workflow';

export const statusOperations: INodeProperties[] = [
  {
    displayName: 'Operation',
    name: 'operation',
    type: 'options',
    noDataExpression: true,
    displayOptions: {
      show: {
        resource: ['status'],
      },
    },
    options: [
      {
        name: 'Get Status',
        value: 'get',
        description: 'Check AuraGo n8n integration status and capabilities',
        action: 'get AuraGo n8n integration status',
      },
    ],
    default: 'get',
  },
];

export const statusFields: INodeProperties[] = [];
