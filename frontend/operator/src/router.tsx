import { createBrowserRouter, Navigate } from 'react-router-dom'
import { lazy } from 'react'
import LoginPage from './pages/LoginPage'
import AppLayout from './components/AppLayout'

const DispatchPage        = lazy(() => import('./pages/DispatchPage'))
const OrderPage           = lazy(() => import('./pages/OrderPage'))
const OperationPage       = lazy(() => import('./pages/OperationPage'))
const DashboardPage       = lazy(() => import('./pages/supervisor/DashboardPage'))
const TRSPage             = lazy(() => import('./pages/supervisor/TRSPage'))
const AlertsPage          = lazy(() => import('./pages/supervisor/AlertsPage'))
const ProgressPage        = lazy(() => import('./pages/supervisor/ProgressPage'))
const NCListPage          = lazy(() => import('./pages/quality/NCListPage'))
const CAPAPage            = lazy(() => import('./pages/quality/CAPAPage'))
const AuditPage           = lazy(() => import('./pages/quality/AuditPage'))
const AsBuiltPage         = lazy(() => import('./pages/quality/AsBuiltPage'))
const QualificationsPage  = lazy(() => import('./pages/admin/QualificationsPage'))
const ToolsPage           = lazy(() => import('./pages/admin/ToolsPage'))
const WorkstationsPage    = lazy(() => import('./pages/admin/WorkstationsPage'))
const RoutingsPage        = lazy(() => import('./pages/admin/RoutingsPage'))

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    path: '/',
    element: <AppLayout />,
    children: [
      { index: true, element: <Navigate to="/dispatch" replace /> },
      { path: 'dispatch', element: <DispatchPage /> },
      { path: 'order/:id', element: <OrderPage /> },
      { path: 'order/:id/operation/:op_id', element: <OperationPage /> },
      { path: 'supervisor', element: <DashboardPage /> },
      { path: 'supervisor/alerts', element: <AlertsPage /> },
      { path: 'supervisor/trs', element: <TRSPage /> },
      { path: 'supervisor/progress', element: <ProgressPage /> },
      { path: 'quality', element: <Navigate to="/quality/nc" replace /> },
      { path: 'quality/nc', element: <NCListPage /> },
      { path: 'quality/capa', element: <CAPAPage /> },
      { path: 'quality/audit', element: <AuditPage /> },
      { path: 'quality/as-built/:ofId', element: <AsBuiltPage /> },
      { path: 'quality/as-built', element: <AsBuiltPage /> },
      { path: 'admin', element: <Navigate to="/admin/qualifications" replace /> },
      { path: 'admin/qualifications', element: <QualificationsPage /> },
      { path: 'admin/tools', element: <ToolsPage /> },
      { path: 'admin/workstations', element: <WorkstationsPage /> },
      { path: 'admin/routings', element: <RoutingsPage /> },
      { path: '*', element: <Navigate to="/dispatch" replace /> },
    ],
  },
])
