import { useEffect, useRef, useCallback } from 'react'

type MessageHandler = (payload: unknown) => void

// useWebSocket connects to the BFF WebSocket relay and lets callers
// subscribe to specific NATS subject patterns.
export function useWebSocket(subjectPatterns: string[], onMessage: MessageHandler) {
  const wsRef = useRef<WebSocket | null>(null)
  const handlersRef = useRef({ subjectPatterns, onMessage })
  handlersRef.current = { subjectPatterns, onMessage }

  const connect = useCallback(() => {
    const token = localStorage.getItem('jwt_token') ?? ''
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${proto}://${window.location.host}/api/v1/ws?token=${encodeURIComponent(token)}`)
    wsRef.current = ws

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data) as { type: string; payload: unknown }
        const { subjectPatterns, onMessage } = handlersRef.current
        const matches = subjectPatterns.some((p) => {
          if (p.endsWith('.*')) return msg.type.startsWith(p.slice(0, -1))
          return msg.type === p
        })
        if (matches) onMessage(msg)
      } catch {
        // ignore malformed frames
      }
    }

    ws.onclose = () => {
      // Reconnect after 3s unless the component unmounted
      setTimeout(() => {
        if (wsRef.current === ws) connect()
      }, 3000)
    }
  }, [])

  useEffect(() => {
    connect()
    return () => {
      const ws = wsRef.current
      if (ws) {
        wsRef.current = null
        ws.close()
      }
    }
  }, [connect])
}
