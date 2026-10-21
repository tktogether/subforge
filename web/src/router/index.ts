import { createRouter, createWebHistory, type Router, type RouterHistory, type RouteRecordRaw } from 'vue-router'

export interface NavItem {
  path: string
  name: string
  title: string
  icon: string
  group: string
}

export interface NavGroup {
  name: string
  items: NavItem[]
}

export const navItems: NavItem[] = [
  { path: '/', name: 'overview', title: '概览', icon: 'mdi-view-dashboard-outline', group: '转换' },
  { path: '/convert', name: 'convert', title: '转换', icon: 'mdi-swap-horizontal', group: '转换' },
  { path: '/pipelines', name: 'pipelines', title: '管线', icon: 'mdi-pipe', group: '转换' },
  { path: '/scripts', name: 'scripts', title: '脚本', icon: 'mdi-script-text-outline', group: '转换' },
  { path: '/airports', name: 'airports', title: '机场', icon: 'mdi-cloud-download-outline', group: '来源' },
  { path: '/self-nodes', name: 'self-nodes', title: '自建节点', icon: 'mdi-server-outline', group: '来源' },
  { path: '/rules', name: 'rules', title: '规则', icon: 'mdi-filter-variant', group: '规则' },
  { path: '/rule-sources', name: 'rule-sources', title: '规则源', icon: 'mdi-file-link-outline', group: '规则' },
  { path: '/groups', name: 'groups', title: '分组', icon: 'mdi-folder-multiple-outline', group: '规则' },
  { path: '/profiles', name: 'profiles', title: '方案', icon: 'mdi-layers-outline', group: '输出' },
  { path: '/subscriptions', name: 'subscriptions', title: '订阅', icon: 'mdi-link-variant', group: '输出' },
  { path: '/tasks', name: 'tasks', title: '任务', icon: 'mdi-clock-outline', group: '运维' },
  { path: '/settings', name: 'settings', title: '设置', icon: 'mdi-cog-outline', group: '运维' },
  { path: '/logs', name: 'logs', title: '日志', icon: 'mdi-text-box-outline', group: '运维' },
]

export function navGroups(): NavGroup[] {
  const groups: NavGroup[] = []
  for (const item of navItems) {
    let group = groups.find((candidate) => candidate.name === item.group)
    if (group === undefined) {
      group = { name: item.group, items: [] }
      groups.push(group)
    }
    group.items.push(item)
  }
  return groups
}

export const routes: RouteRecordRaw[] = [
  { path: '/', name: 'overview', component: () => import('../pages/OverviewPage.vue'), meta: { title: '概览' } },
  { path: '/convert', name: 'convert', component: () => import('../pages/ConvertPage.vue'), meta: { title: '转换' } },
  { path: '/pipelines', name: 'pipelines', component: () => import('../pages/PipelinesPage.vue'), meta: { title: '管线' } },
  { path: '/scripts', name: 'scripts', component: () => import('../pages/ScriptsPage.vue'), meta: { title: '脚本' } },
  { path: '/airports', name: 'airports', component: () => import('../pages/AirportsPage.vue'), meta: { title: '机场' } },
  { path: '/self-nodes', name: 'self-nodes', component: () => import('../pages/SelfNodesPage.vue'), meta: { title: '自建节点' } },
  { path: '/rules', name: 'rules', component: () => import('../pages/RulesPage.vue'), meta: { title: '规则' } },
  { path: '/rule-sources', name: 'rule-sources', component: () => import('../pages/RuleSourcesPage.vue'), meta: { title: '规则源' } },
  { path: '/groups', name: 'groups', component: () => import('../pages/GroupsPage.vue'), meta: { title: '分组' } },
  { path: '/profiles', name: 'profiles', component: () => import('../pages/ProfilesPage.vue'), meta: { title: '方案' } },
  { path: '/subscriptions', name: 'subscriptions', component: () => import('../pages/SubscriptionsPage.vue'), meta: { title: '订阅' } },
  { path: '/tasks', name: 'tasks', component: () => import('../pages/TasksPage.vue'), meta: { title: '任务' } },
  { path: '/settings', name: 'settings', component: () => import('../pages/SettingsPage.vue'), meta: { title: '设置' } },
  { path: '/logs', name: 'logs', component: () => import('../pages/LogsPage.vue'), meta: { title: '日志' } },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('../pages/NotFoundPage.vue'), meta: { title: '页面不存在' } },
]

export function createAppRouter(history?: RouterHistory): Router {
  return createRouter({
    history: history ?? createWebHistory(),
    routes,
  })
}
