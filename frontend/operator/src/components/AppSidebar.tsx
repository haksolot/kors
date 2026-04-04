import { useLocation, Link, useNavigate } from 'react-router-dom'
import {
  ClipboardList, LayoutDashboard, Bell, BarChart3, TrendingUp,
  AlertTriangle, ShieldCheck, FileSearch, FileText, UserCheck,
  Wrench, Monitor, GitBranch, LogOut, Sun, Moon, Factory,
} from 'lucide-react'
import {
  Sidebar, SidebarContent, SidebarFooter, SidebarGroup,
  SidebarGroupContent, SidebarGroupLabel, SidebarHeader,
  SidebarMenu, SidebarMenuButton, SidebarMenuItem, SidebarSeparator,
} from '@/components/ui/sidebar'
import { useTheme } from '@/lib/theme'

interface NavItem {
  label: string
  to: string
  icon: React.ElementType
  match?: (path: string) => boolean
}

const operatorNav: NavItem[] = [
  { label: 'Mes OF', to: '/dispatch', icon: ClipboardList },
]

const supervisorNav: NavItem[] = [
  { label: 'Vue live', to: '/supervisor', icon: LayoutDashboard, match: (p) => p === '/supervisor' },
  { label: 'Alertes', to: '/supervisor/alerts', icon: Bell },
  { label: 'TRS / OEE', to: '/supervisor/trs', icon: BarChart3 },
  { label: 'Avancement', to: '/supervisor/progress', icon: TrendingUp },
]

const qualityNav: NavItem[] = [
  { label: 'Non-conformités', to: '/quality/nc', icon: AlertTriangle },
  { label: 'CAPA', to: '/quality/capa', icon: ShieldCheck },
  { label: 'Audit trail', to: '/quality/audit', icon: FileSearch },
  { label: 'As-Built', to: '/quality/as-built', icon: FileText },
]

const adminNav: NavItem[] = [
  { label: 'Habilitations', to: '/admin/qualifications', icon: UserCheck },
  { label: 'Outillages', to: '/admin/tools', icon: Wrench },
  { label: 'Postes', to: '/admin/workstations', icon: Monitor },
  { label: 'Gammes', to: '/admin/routings', icon: GitBranch },
]

function NavGroup({ label, items }: { label: string; items: NavItem[] }) {
  const { pathname } = useLocation()
  return (
    <SidebarGroup>
      <SidebarGroupLabel>{label}</SidebarGroupLabel>
      <SidebarGroupContent>
        <SidebarMenu>
          {items.map((item) => {
            const isActive = item.match ? item.match(pathname) : pathname.startsWith(item.to)
            return (
              <SidebarMenuItem key={item.to}>
                <SidebarMenuButton
                  render={<Link to={item.to} />}
                  isActive={isActive}
                  tooltip={item.label}
                >
                  <item.icon />
                  <span>{item.label}</span>
                </SidebarMenuButton>
              </SidebarMenuItem>
            )
          })}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}

export function AppSidebar() {
  const { theme, toggle } = useTheme()
  const navigate = useNavigate()

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" render={<Link to="/dispatch" />}>
              <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                <Factory className="size-4" />
              </div>
              <div className="flex flex-col gap-0.5 leading-none">
                <span className="font-semibold">KORS MES</span>
                <span className="text-xs text-muted-foreground">Bord de ligne</span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        <NavGroup label="Opérateur" items={operatorNav} />
        <SidebarSeparator />
        <NavGroup label="Supervision" items={supervisorNav} />
        <SidebarSeparator />
        <NavGroup label="Qualité" items={qualityNav} />
        <SidebarSeparator />
        <NavGroup label="Administration" items={adminNav} />
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton onClick={toggle} tooltip={theme === 'dark' ? 'Mode clair' : 'Mode sombre'}>
              {theme === 'dark' ? <Sun /> : <Moon />}
              <span>{theme === 'dark' ? 'Mode clair' : 'Mode sombre'}</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem>
            <SidebarMenuButton
              onClick={() => { localStorage.removeItem('jwt_token'); navigate('/login') }}
              tooltip="Déconnexion"
              className="text-destructive hover:text-destructive"
            >
              <LogOut />
              <span>Déconnexion</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  )
}
