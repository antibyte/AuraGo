import type { INodeProperties } from 'n8n-workflow';

export const missionOperations: INodeProperties[] = [
  {
    displayName: 'Operation',
    name: 'operation',
    type: 'options',
    noDataExpression: true,
    displayOptions: {
      show: {
        resource: ['mission'],
      },
    },
    options: [
      {
        name: 'Create',
        value: 'create',
        description: 'Create a new mission',
        action: 'Create a new mission',
      },
      {
        name: 'Create and Run',
        value: 'createAndRun',
        description: 'Create and immediately execute a mission',
        action: 'Create and run a mission',
      },
    ],
    default: 'create',
  },
];

export const missionFields: INodeProperties[] = [
  {
    displayName: 'Mission Name',
    name: 'missionName',
    type: 'string',
    default: '',
    required: true,
    displayOptions: {
      show: {
        resource: ['mission'],
        operation: ['create', 'createAndRun'],
      },
    },
    description: 'Name of the mission',
  },
  {
    displayName: 'Description',
    name: 'description',
    type: 'string',
    typeOptions: {
      rows: 3,
    },
    default: '',
    displayOptions: {
      show: {
        resource: ['mission'],
        operation: ['create', 'createAndRun'],
      },
    },
    description: 'Description of what the mission should accomplish',
  },
  {
    displayName: 'Trigger Type',
    name: 'triggerType',
    type: 'options',
    options: [
      {
        name: 'Manual',
        value: 'manual',
      },
      {
        name: 'Webhook',
        value: 'webhook',
      },
      {
        name: 'Schedule',
        value: 'schedule',
      },
    ],
    default: 'manual',
    displayOptions: {
      show: {
        resource: ['mission'],
        operation: ['create', 'createAndRun'],
      },
    },
    description: 'How the mission should be triggered',
  },
  {
    displayName: 'Steps',
    name: 'steps',
    type: 'json',
    default: '[]',
    typeOptions: {
      rows: 8,
    },
    displayOptions: {
      show: {
        resource: ['mission'],
        operation: ['create', 'createAndRun'],
      },
    },
    description: 'Mission steps as JSON array (advanced)',
  },
  {
    displayName: 'Schedule',
    name: 'schedule',
    type: 'string',
    default: '',
    placeholder: '0 9 * * *',
    displayOptions: {
      show: {
        resource: ['mission'],
        operation: ['create', 'createAndRun'],
        triggerType: ['schedule'],
      },
    },
    description: 'Cron expression for scheduled missions, for example 0 9 * * *',
  },
];
