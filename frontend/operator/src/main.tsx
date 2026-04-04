import { StrictMode, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ThemeProvider } from '@/lib/theme'
import './index.css'
import { router } from './router'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider>
      <TooltipProvider>
        <Suspense
          fallback={
            <div className="min-h-screen bg-background flex items-center justify-center">
              <p className="text-muted-foreground">Chargement…</p>
            </div>
          }
        >
          <RouterProvider router={router} />
        </Suspense>
      </TooltipProvider>
    </ThemeProvider>
  </StrictMode>,
)
