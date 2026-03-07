"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.ListResourcesDocument = exports.GetResourceDocument = exports.TransitionResourceDocument = exports.CreateResourceDocument = exports.RegisterResourceTypeDocument = void 0;
exports.getSdk = getSdk;
const graphql_1 = require("graphql");
const graphql_tag_1 = __importDefault(require("graphql-tag"));
exports.RegisterResourceTypeDocument = (0, graphql_tag_1.default) `
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
exports.CreateResourceDocument = (0, graphql_tag_1.default) `
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
exports.TransitionResourceDocument = (0, graphql_tag_1.default) `
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
exports.GetResourceDocument = (0, graphql_tag_1.default) `
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
exports.ListResourcesDocument = (0, graphql_tag_1.default) `
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
const defaultWrapper = (action, _operationName, _operationType, _variables) => action();
const RegisterResourceTypeDocumentString = (0, graphql_1.print)(exports.RegisterResourceTypeDocument);
const CreateResourceDocumentString = (0, graphql_1.print)(exports.CreateResourceDocument);
const TransitionResourceDocumentString = (0, graphql_1.print)(exports.TransitionResourceDocument);
const GetResourceDocumentString = (0, graphql_1.print)(exports.GetResourceDocument);
const ListResourcesDocumentString = (0, graphql_1.print)(exports.ListResourcesDocument);
function getSdk(client, withWrapper = defaultWrapper) {
    return {
        RegisterResourceType(variables, requestHeaders) {
            return withWrapper((wrappedRequestHeaders) => client.rawRequest(RegisterResourceTypeDocumentString, variables, { ...requestHeaders, ...wrappedRequestHeaders }), 'RegisterResourceType', 'mutation', variables);
        },
        CreateResource(variables, requestHeaders) {
            return withWrapper((wrappedRequestHeaders) => client.rawRequest(CreateResourceDocumentString, variables, { ...requestHeaders, ...wrappedRequestHeaders }), 'CreateResource', 'mutation', variables);
        },
        TransitionResource(variables, requestHeaders) {
            return withWrapper((wrappedRequestHeaders) => client.rawRequest(TransitionResourceDocumentString, variables, { ...requestHeaders, ...wrappedRequestHeaders }), 'TransitionResource', 'mutation', variables);
        },
        GetResource(variables, requestHeaders) {
            return withWrapper((wrappedRequestHeaders) => client.rawRequest(GetResourceDocumentString, variables, { ...requestHeaders, ...wrappedRequestHeaders }), 'GetResource', 'query', variables);
        },
        ListResources(variables, requestHeaders) {
            return withWrapper((wrappedRequestHeaders) => client.rawRequest(ListResourcesDocumentString, variables, { ...requestHeaders, ...wrappedRequestHeaders }), 'ListResources', 'query', variables);
        }
    };
}
