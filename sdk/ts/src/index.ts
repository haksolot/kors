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

  /**
   * Save a file to KORS MinIO storage
   */
  async saveFile(fileName: string, content: Buffer | Uint8Array | string): Promise<string> {
    let base64Content: string;
    
    if (typeof content === 'string') {
      base64Content = content;
    } else if (content instanceof Buffer) {
      base64Content = content.toString('base64');
    } else {
      base64Content = Buffer.from(content).toString('base64');
    }

    const mutation = `
      mutation UploadFile($input: UploadFileInput!) {
        uploadFile(input: $input) {
          success
          url
          error {
            message
          }
        }
      }
    `;

    const response: any = await this.client.request(mutation, {
      input: {
        fileName,
        fileContent: base64Content,
        contentType: 'application/octet-stream',
      },
    });

    if (!response.uploadFile.success) {
      throw new Error(`KORS upload error: ${response.uploadFile.error?.message || 'Unknown error'}`);
    }

    return response.uploadFile.url;
  }
}

// Re-export all generated types for the consumer
export * from './generated/kors';
