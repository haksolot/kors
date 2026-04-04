import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Factory, KeyRound } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

export default function LoginPage() {
  const [token, setToken] = useState('')
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = token.trim()
    if (!trimmed) { setError('Token requis'); return }
    localStorage.setItem('jwt_token', trimmed)
    navigate('/dispatch')
  }

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-6">
      <div className="w-full max-w-sm flex flex-col items-center gap-6">
        <div className="flex flex-col items-center gap-2">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <Factory className="size-6" />
          </div>
          <h1 className="text-2xl font-bold">KORS MES</h1>
          <p className="text-muted-foreground text-sm text-center">
            Bord de ligne — Identifiez-vous pour accéder à vos OF
          </p>
        </div>

        <Card className="w-full">
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <KeyRound className="size-4" />
              Jeton JWT
            </CardTitle>
            <CardDescription>
              Collez votre token d'accès ci-dessous
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="token" className="sr-only">Token JWT</Label>
                <Textarea
                  id="token"
                  rows={5}
                  placeholder="eyJhbGciOiJSUzI1Ni..."
                  value={token}
                  onChange={(e) => { setToken(e.target.value); setError(null) }}
                  className="font-mono text-xs resize-none"
                  autoComplete="off"
                  spellCheck={false}
                />
                {error && <p className="text-destructive text-sm">{error}</p>}
              </div>
              <Button type="submit" size="lg" className="w-full">
                Connexion
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
