# KORS SDK for Python

Le SDK KORS pour Python, moderne, asynchrone et typé avec Pydantic.

## 1. Installation

```bash
pip install kors-sdk
```

## 2. Initialisation

```python
import asyncio
from kors_client import KorsClient

async def main():
    async with KorsClient(endpoint="http://localhost:8080/query", token="JWT") as client:
        # Utilisation via l'objet API typé
        resp = await client.api.get_resource(id="UUID")
        print(resp.resource.state)

if __name__ == "__main__":
    asyncio.run(main())
```

## 3. Exemples

### Création d'une ressource
```python
from kors_sdk.input_types import CreateResourceInput

input_data = CreateResourceInput(
    typeName="tool",
    initialState="idle",
    metadata={"serial": "SN-789"}
)

resp = await client.api.create_resource(input=input_data)
if resp.create_resource.success:
    print(f"Created: {resp.create_resource.resource.id}")
```

---

## Maintenance du SDK
Pour mettre à jour le SDK après une modification du schéma :
```bash
ariadne-codegen
```
