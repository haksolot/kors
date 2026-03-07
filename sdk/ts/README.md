# KORS SDK for TypeScript/JavaScript

Le SDK KORS pour le web et Node.js, entièrement typé et asynchrone.

## 1. Installation

```bash
npm install @kors-project/kors-sdk
```

## 2. Initialisation

```typescript
import { KorsClient } from '@kors-project/kors-sdk';

const client = new KorsClient({
  endpoint: 'http://localhost:8080/query',
  token: 'YOUR_JWT_TOKEN'
});
```

## 3. Exemples

### Création d'une ressource
```typescript
const { data } = await client.sdk.CreateResource({
  input: {
    typeName: 'tool',
    initialState: 'idle',
    metadata: { serial: 'SN-456' }
  }
});

if (data?.createResource.success) {
  console.log('ID:', data.createResource.resource.id);
}
```

### Transition d'état
```typescript
await client.sdk.TransitionResource({
  input: {
    resourceId: 'UUID',
    toState: 'maintenance'
  }
});
```

---

## Maintenance du SDK
Pour régénérer les types après une modification du schéma :
```bash
npm run generate
```
