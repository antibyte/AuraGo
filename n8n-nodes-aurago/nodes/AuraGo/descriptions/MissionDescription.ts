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
      {
        name: 'Delete',
        value: 'delete',
        description: 'Delete an existing mission',
        action: 'Delete a mission',
      },
      {
        name: 'Get',
        value: 'get',
        description: 'Get mission details',
        action: 'Get a mission',
      },
      {
        name: 'List',
        value: 'list',
        description: 'List all missions',
        action: 'List missions',
      },
      {
        name: 'Run',
        value: 'run',
        description: 'Queue an existing mission for execution',
        action: 'Run a mission',
      },
      {
        name: 'Update',
        value: 'update',
        description: 'Update an existing mission',
        action: 'Update a mission',
      },
    ],
    default: 'create',
  },
];

export const missionFields: INodeProperties[] = [
  {
    displayName: 'Mission ID',
    name: 'missionId',
    type: 'string',
    default: '',
    required: true,
    displayOptions: {
      show: {
        resource: ['mission'],
        operation: ['get', 'update', 'delete', 'run'],
      },
    },
    description: 'ID of the mission to manage',
  },
  {
    displayName: 'Mission Name',
    name: 'missionName',
    type: 'string',
    default: '',
    required: true,
    displayOptions: {
      show: {
        resource: ['mission'],
        operation: ['create', 'createAndRun', 'update'],
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
        operation: ['create', 'createAndRun', 'update'],
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
        operation: ['create', 'createAndRun', 'update'],
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
        operation: ['create', 'createAndRun', 'update'],
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
        operation: ['create', 'createAndRun', 'update'],
        triggerType: ['schedule'],
      },
    },
    description: 'Cron expression for scheduled missions, for example 0 9 * * *',
  },
  {
    displayName: 'Priority',
    name: 'priority',
    type: 'options',
    options: [
      { name: 'Low', value: 'low' },
      { name: 'Medium', value: 'medium' },
      { name: 'High', value: 'high' },
    ],
    default: 'medium',
    displayOptions: {
      show: {
        resource: ['mission'],
        operation: ['create', 'createAndRun', 'update'],
      },
    },
    description: 'Mission priority',
  },
  {
    displayName: 'Enabled',
    name: 'enabled',
    type: 'boolean',
    default: true,
    displayOptions: {
      show: {
        resource: ['mission'],
        operation: ['update'],
      },
    },
    description: 'Whether the mission is enabled',
  },
];
