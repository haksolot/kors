import { GraphQLClient, RequestOptions } from 'graphql-request';
import { GraphQLError, print } from 'graphql'
import gql from 'graphql-tag';
export type Maybe<T> = T | null;
export type InputMaybe<T> = Maybe<T>;
export type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]?: Maybe<T[SubKey]> };
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]: Maybe<T[SubKey]> };
export type MakeEmpty<T extends { [key: string]: unknown }, K extends keyof T> = { [_ in K]?: never };
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
type GraphQLClientRequestHeaders = RequestOptions['requestHeaders'];
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: { input: string; output: string; }
  String: { input: string; output: string; }
  Boolean: { input: boolean; output: boolean; }
  Int: { input: number; output: number; }
  Float: { input: number; output: number; }
  DateTime: { input: string; output: string; }
  JSON: { input: { [key: string]: any }; output: { [key: string]: any }; }
  UUID: { input: string; output: string; }
};

export type CreateResourceInput = {
  initialState: Scalars['String']['input'];
  metadata?: InputMaybe<Scalars['JSON']['input']>;
  typeName: Scalars['String']['input'];
};

export type CreateRevisionInput = {
  fileContent?: InputMaybe<Scalars['String']['input']>;
  fileName?: InputMaybe<Scalars['String']['input']>;
  resourceId: Scalars['UUID']['input'];
};

export type Event = {
  __typename?: 'Event';
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['UUID']['output'];
  identity: Identity;
  natsMessageId?: Maybe<Scalars['UUID']['output']>;
  payload: Scalars['JSON']['output'];
  resource?: Maybe<Resource>;
  type: Scalars['String']['output'];
};

export type GrantPermissionInput = {
  action: Scalars['String']['input'];
  expiresAt?: InputMaybe<Scalars['DateTime']['input']>;
  identityId: Scalars['UUID']['input'];
  resourceId?: InputMaybe<Scalars['UUID']['input']>;
  resourceTypeId?: InputMaybe<Scalars['UUID']['input']>;
};

export type Identity = {
  __typename?: 'Identity';
  createdAt: Scalars['DateTime']['output'];
  externalId?: Maybe<Scalars['String']['output']>;
  id: Scalars['UUID']['output'];
  metadata?: Maybe<Scalars['JSON']['output']>;
  name: Scalars['String']['output'];
  type: Scalars['String']['output'];
  updatedAt: Scalars['DateTime']['output'];
};

export type Mutation = {
  __typename?: 'Mutation';
  createResource: ResourceResult;
  createRevision: RevisionResult;
  grantPermission: PermissionResult;
  provisionModule: ProvisioningResult;
  registerResourceType: ResourceTypeResult;
  transitionResource: ResourceResult;
};


export type MutationCreateResourceArgs = {
  input: CreateResourceInput;
};


export type MutationCreateRevisionArgs = {
  input: CreateRevisionInput;
};


export type MutationGrantPermissionArgs = {
  input: GrantPermissionInput;
};


export type MutationProvisionModuleArgs = {
  moduleName: Scalars['String']['input'];
};


export type MutationRegisterResourceTypeArgs = {
  input: RegisterResourceTypeInput;
};


export type MutationTransitionResourceArgs = {
  input: TransitionResourceInput;
};

/** MutationError represents a business error occurred during a mutation. */
export type MutationError = {
  __typename?: 'MutationError';
  code: Scalars['String']['output'];
  message: Scalars['String']['output'];
};

/** PageInfo contains metadata about a paginated result. */
export type PageInfo = {
  __typename?: 'PageInfo';
  endCursor?: Maybe<Scalars['String']['output']>;
  hasNextPage: Scalars['Boolean']['output'];
  hasPreviousPage: Scalars['Boolean']['output'];
  startCursor?: Maybe<Scalars['String']['output']>;
};

export type Permission = {
  __typename?: 'Permission';
  action: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  expiresAt?: Maybe<Scalars['DateTime']['output']>;
  id: Scalars['UUID']['output'];
  identity: Identity;
  resource?: Maybe<Resource>;
  resourceType?: Maybe<ResourceType>;
};

export type PermissionResult = {
  __typename?: 'PermissionResult';
  error?: Maybe<MutationError>;
  permission?: Maybe<Permission>;
  success: Scalars['Boolean']['output'];
};

export type ProvisioningResult = {
  __typename?: 'ProvisioningResult';
  error?: Maybe<MutationError>;
  moduleName?: Maybe<Scalars['String']['output']>;
  password?: Maybe<Scalars['String']['output']>;
  schema?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
  username?: Maybe<Scalars['String']['output']>;
};

export type Query = {
  __typename?: 'Query';
  resource?: Maybe<Resource>;
  resourceType?: Maybe<ResourceType>;
  resourceTypes: Array<ResourceType>;
  resources: ResourceConnection;
};


export type QueryResourceArgs = {
  id: Scalars['UUID']['input'];
};


export type QueryResourceTypeArgs = {
  name: Scalars['String']['input'];
};


export type QueryResourcesArgs = {
  after?: InputMaybe<Scalars['String']['input']>;
  first?: InputMaybe<Scalars['Int']['input']>;
  typeName?: InputMaybe<Scalars['String']['input']>;
};

export type RegisterResourceTypeInput = {
  description?: InputMaybe<Scalars['String']['input']>;
  jsonSchema: Scalars['JSON']['input'];
  name: Scalars['String']['input'];
  transitions: Scalars['JSON']['input'];
};

export type Resource = {
  __typename?: 'Resource';
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['UUID']['output'];
  metadata: Scalars['JSON']['output'];
  revisions: Array<Revision>;
  state: Scalars['String']['output'];
  type: ResourceType;
  updatedAt: Scalars['DateTime']['output'];
};

export type ResourceConnection = {
  __typename?: 'ResourceConnection';
  edges: Array<ResourceEdge>;
  pageInfo: PageInfo;
  totalCount: Scalars['Int']['output'];
};

export type ResourceEdge = {
  __typename?: 'ResourceEdge';
  cursor: Scalars['String']['output'];
  node: Resource;
};

export type ResourceResult = {
  __typename?: 'ResourceResult';
  error?: Maybe<MutationError>;
  resource?: Maybe<Resource>;
  success: Scalars['Boolean']['output'];
};

export type ResourceType = {
  __typename?: 'ResourceType';
  createdAt: Scalars['DateTime']['output'];
  description?: Maybe<Scalars['String']['output']>;
  id: Scalars['UUID']['output'];
  jsonSchema: Scalars['JSON']['output'];
  name: Scalars['String']['output'];
  transitions: Scalars['JSON']['output'];
  updatedAt: Scalars['DateTime']['output'];
};

export type ResourceTypeResult = {
  __typename?: 'ResourceTypeResult';
  error?: Maybe<MutationError>;
  resourceType?: Maybe<ResourceType>;
  success: Scalars['Boolean']['output'];
};

export type Revision = {
  __typename?: 'Revision';
  createdAt: Scalars['DateTime']['output'];
  downloadUrl?: Maybe<Scalars['String']['output']>;
  filePath?: Maybe<Scalars['String']['output']>;
  id: Scalars['UUID']['output'];
  identity: Identity;
  resource: Resource;
  snapshot: Scalars['JSON']['output'];
};

export type RevisionResult = {
  __typename?: 'RevisionResult';
  error?: Maybe<MutationError>;
  revision?: Maybe<Revision>;
  success: Scalars['Boolean']['output'];
};

export type Subscription = {
  __typename?: 'Subscription';
  eventWasPublished: Event;
  resourceEvents: Event;
};


export type SubscriptionResourceEventsArgs = {
  resourceId: Scalars['UUID']['input'];
};

export type TransitionResourceInput = {
  metadata?: InputMaybe<Scalars['JSON']['input']>;
  resourceId: Scalars['UUID']['input'];
  toState: Scalars['String']['input'];
};

export type RegisterResourceTypeMutationVariables = Exact<{
  input: RegisterResourceTypeInput;
}>;


export type RegisterResourceTypeMutation = { __typename?: 'Mutation', registerResourceType: { __typename?: 'ResourceTypeResult', success: boolean, resourceType?: { __typename?: 'ResourceType', id: string, name: string } | null, error?: { __typename?: 'MutationError', code: string, message: string } | null } };

export type CreateResourceMutationVariables = Exact<{
  input: CreateResourceInput;
}>;


export type CreateResourceMutation = { __typename?: 'Mutation', createResource: { __typename?: 'ResourceResult', success: boolean, resource?: { __typename?: 'Resource', id: string, state: string } | null, error?: { __typename?: 'MutationError', message: string } | null } };

export type TransitionResourceMutationVariables = Exact<{
  input: TransitionResourceInput;
}>;


export type TransitionResourceMutation = { __typename?: 'Mutation', transitionResource: { __typename?: 'ResourceResult', success: boolean, resource?: { __typename?: 'Resource', id: string, state: string } | null, error?: { __typename?: 'MutationError', code: string, message: string } | null } };

export type GetResourceQueryVariables = Exact<{
  id: Scalars['UUID']['input'];
}>;


export type GetResourceQuery = { __typename?: 'Query', resource?: { __typename?: 'Resource', id: string, state: string, metadata: { [key: string]: any }, type: { __typename?: 'ResourceType', name: string } } | null };

export type ListResourcesQueryVariables = Exact<{
  first?: InputMaybe<Scalars['Int']['input']>;
  after?: InputMaybe<Scalars['String']['input']>;
  typeName?: InputMaybe<Scalars['String']['input']>;
}>;


export type ListResourcesQuery = { __typename?: 'Query', resources: { __typename?: 'ResourceConnection', totalCount: number, edges: Array<{ __typename?: 'ResourceEdge', cursor: string, node: { __typename?: 'Resource', id: string, state: string } }>, pageInfo: { __typename?: 'PageInfo', hasNextPage: boolean, endCursor?: string | null } } };


export const RegisterResourceTypeDocument = gql`
    mutation RegisterResourceType($input: RegisterResourceTypeInput!) {
  registerResourceType(input: $input) {
    success
    resourceType {
      id
      name
    }
    error {
      code
      message
    }
  }
}
    `;
export const CreateResourceDocument = gql`
    mutation CreateResource($input: CreateResourceInput!) {
  createResource(input: $input) {
    success
    resource {
      id
      state
    }
    error {
      message
    }
  }
}
    `;
export const TransitionResourceDocument = gql`
    mutation TransitionResource($input: TransitionResourceInput!) {
  transitionResource(input: $input) {
    success
    resource {
      id
      state
    }
    error {
      code
      message
    }
  }
}
    `;
export const GetResourceDocument = gql`
    query GetResource($id: UUID!) {
  resource(id: $id) {
    id
    state
    metadata
    type {
      name
    }
  }
}
    `;
export const ListResourcesDocument = gql`
    query ListResources($first: Int, $after: String, $typeName: String) {
  resources(first: $first, after: $after, typeName: $typeName) {
    totalCount
    edges {
      cursor
      node {
        id
        state
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}
    `;

export type SdkFunctionWrapper = <T>(action: (requestHeaders?:Record<string, string>) => Promise<T>, operationName: string, operationType?: string, variables?: any) => Promise<T>;


const defaultWrapper: SdkFunctionWrapper = (action, _operationName, _operationType, _variables) => action();
const RegisterResourceTypeDocumentString = print(RegisterResourceTypeDocument);
const CreateResourceDocumentString = print(CreateResourceDocument);
const TransitionResourceDocumentString = print(TransitionResourceDocument);
const GetResourceDocumentString = print(GetResourceDocument);
const ListResourcesDocumentString = print(ListResourcesDocument);
export function getSdk(client: GraphQLClient, withWrapper: SdkFunctionWrapper = defaultWrapper) {
  return {
    RegisterResourceType(variables: RegisterResourceTypeMutationVariables, requestHeaders?: GraphQLClientRequestHeaders): Promise<{ data: RegisterResourceTypeMutation; errors?: GraphQLError[]; extensions?: any; headers: Headers; status: number; }> {
        return withWrapper((wrappedRequestHeaders) => client.rawRequest<RegisterResourceTypeMutation>(RegisterResourceTypeDocumentString, variables, {...requestHeaders, ...wrappedRequestHeaders}), 'RegisterResourceType', 'mutation', variables);
    },
    CreateResource(variables: CreateResourceMutationVariables, requestHeaders?: GraphQLClientRequestHeaders): Promise<{ data: CreateResourceMutation; errors?: GraphQLError[]; extensions?: any; headers: Headers; status: number; }> {
        return withWrapper((wrappedRequestHeaders) => client.rawRequest<CreateResourceMutation>(CreateResourceDocumentString, variables, {...requestHeaders, ...wrappedRequestHeaders}), 'CreateResource', 'mutation', variables);
    },
    TransitionResource(variables: TransitionResourceMutationVariables, requestHeaders?: GraphQLClientRequestHeaders): Promise<{ data: TransitionResourceMutation; errors?: GraphQLError[]; extensions?: any; headers: Headers; status: number; }> {
        return withWrapper((wrappedRequestHeaders) => client.rawRequest<TransitionResourceMutation>(TransitionResourceDocumentString, variables, {...requestHeaders, ...wrappedRequestHeaders}), 'TransitionResource', 'mutation', variables);
    },
    GetResource(variables: GetResourceQueryVariables, requestHeaders?: GraphQLClientRequestHeaders): Promise<{ data: GetResourceQuery; errors?: GraphQLError[]; extensions?: any; headers: Headers; status: number; }> {
        return withWrapper((wrappedRequestHeaders) => client.rawRequest<GetResourceQuery>(GetResourceDocumentString, variables, {...requestHeaders, ...wrappedRequestHeaders}), 'GetResource', 'query', variables);
    },
    ListResources(variables?: ListResourcesQueryVariables, requestHeaders?: GraphQLClientRequestHeaders): Promise<{ data: ListResourcesQuery; errors?: GraphQLError[]; extensions?: any; headers: Headers; status: number; }> {
        return withWrapper((wrappedRequestHeaders) => client.rawRequest<ListResourcesQuery>(ListResourcesDocumentString, variables, {...requestHeaders, ...wrappedRequestHeaders}), 'ListResources', 'query', variables);
    }
  };
}
export type Sdk = ReturnType<typeof getSdk>;