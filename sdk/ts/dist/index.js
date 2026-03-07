"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __exportStar = (this && this.__exportStar) || function(m, exports) {
    for (var p in m) if (p !== "default" && !Object.prototype.hasOwnProperty.call(exports, p)) __createBinding(exports, m, p);
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.KorsClient = void 0;
const graphql_request_1 = require("graphql-request");
const kors_1 = require("./generated/kors");
/**
 * KORS SDK Client for TypeScript/JavaScript
 */
class KorsClient {
    client;
    sdk;
    constructor(config) {
        this.client = new graphql_request_1.GraphQLClient(config.endpoint, {
            headers: config.token
                ? { Authorization: `Bearer ${config.token}` }
                : {},
        });
        this.sdk = (0, kors_1.getSdk)(this.client);
    }
    /**
     * Update the authentication token
     */
    setToken(token) {
        this.client.setHeader('Authorization', `Bearer ${token}`);
    }
}
exports.KorsClient = KorsClient;
// Re-export all generated types for the consumer
__exportStar(require("./generated/kors"), exports);
