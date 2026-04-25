import type { INodeProperties } from 'n8n-workflow';

export const memoryOperations: INodeProperties[] = [
  {
    displayName: 'Operation',
    name: 'operation',
    type: 'options',
    noDataExpression: true,
    displayOptions: {
      show: {
        resource: ['memory'],
      },
    },
    options: [
      {
        name: 'Search',
        value: 'search',
        description: 'Search agent memory',
        action: 'Search agent memory',
      },
      {
        name: 'Store',
        value: 'store',
        description: 'Store information in agent memory',
        action: 'Store information in agent memory',
      },
    ],
    default: 'search',
  },
];

export const memoryFields: INodeProperties[] = [
  // Search Memory
  {
    displayName: 'Query',
    name: 'query',
    type: 'string',
    default: '',
    required: true,
    displayOptions: {
      show: {
        resource: ['memory'],
        operation: ['search'],
      },
    },
    description: 'Search query for memory retrieval',
  },
  {
    displayName: 'Memory Type',
    name: 'memoryType',
    type: 'options',
    options: [
      {
        name: 'Short Term (Chat History)',
        value: 'short_term',
      },
      {
        name: 'Long Term (Vector DB)',
        value: 'long_term',
      },
      {
        name: 'Knowledge Graph',
        value: 'knowledge_graph',
      },
      {
        name: 'All Types',
        value: 'all',
      },
    ],
    default: 'all',
    displayOptions: {
      show: {
        resource: ['memory'],
        operation: ['search'],
      },
    },
    description: 'Type of memory to search',
  },
  {
    displayName: 'Limit',
    name: 'limit',
    type: 'number',
    typeOptions: {
      minValue: 1,
      maxValue: 100,
    },
    default: 50,
    displayOptions: {
      show: {
        resource: ['memory'],
        operation: ['search'],
      },
    },
    description: 'Max number of results to return',
  },

  // Store Memory
  {
    displayName: 'Content',
    name: 'content',
    type: 'string',
    typeOptions: {
      rows: 4,
    },
    default: '',
    required: true,
    displayOptions: {
      show: {
        resource: ['memory'],
        operation: ['store'],
      },
    },
    description: 'Content to store in memory',
  },
  {
    displayName: 'Storage Type',
    name: 'storageType',
    type: 'options',
    options: [
      {
        name: 'Short Term',
        value: 'short_term',
      },
      {
        name: 'Long Term',
        value: 'long_term',
      },
      {
        name: 'Core Memory',
        value: 'core',
      },
    ],
    default: 'long_term',
    displayOptions: {
      show: {
        resource: ['memory'],
        operation: ['store'],
      },
    },
    description: 'Where to store the memory',
  },
  {
    displayName: 'Metadata',
    name: 'metadata',
    type: 'json',
    default: '{}',
    typeOptions: {
      rows: 4,
    },
    displayOptions: {
      show: {
        resource: ['memory'],
        operation: ['store'],
      },
    },
    description: 'Optional metadata as JSON',
  },
];
