import { getSdk } from './generated/kors';
export interface KorsConfig {
    endpoint: string;
    token?: string;
}
/**
 * KORS SDK Client for TypeScript/JavaScript
 */
export declare class KorsClient {
    private client;
    sdk: ReturnType<typeof getSdk>;
    constructor(config: KorsConfig);
    /**
     * Update the authentication token
     */
    setToken(token: string): void;
}
export * from './generated/kors';
