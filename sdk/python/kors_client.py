from typing import Optional
from .kors_sdk.client import KorsClient as GeneratedClient

class KorsClient:
    """
    KORS SDK Client for Python (Async)
    """
    def __init__(self, endpoint: str, token: Optional[str] = None):
        headers = {}
        if token:
            headers["Authorization"] = f"Bearer {token}"
        
        self.client = GeneratedClient(url=endpoint, headers=headers)

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.client.close()

    @property
    def api(self) -> GeneratedClient:
        """Access to the raw generated API methods."""
        return self.client
