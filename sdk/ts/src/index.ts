import { GraphQLClient } from 'graphql-request';
import { getSdk } from './generated/kors';

export interface KorsConfig {
  endpoint: string;
  token?: string;
}

/**
 * KORS SDK Client for TypeScript/JavaScript
 */
export class KorsClient {
  private client: GraphQLClient;
  public sdk: ReturnType<typeof getSdk>;

  constructor(config: KorsConfig) {
    this.client = new GraphQLClient(config.endpoint, {
      headers: config.token 
        ? { Authorization: `Bearer ${config.token}` } 
        : {},
    });
    this.sdk = getSdk(this.client);
  }

  /**
   * Update the authentication token
   */
  setToken(token: string) {
    this.client.setHeader('Authorization', `Bearer ${token}`);
  }
}

// Re-export all generated types for the consumer
export * from './generated/kors';
