import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import AppLayout from '../layouts/AppLayout.vue'
import DashboardPage from '../pages/DashboardPage.vue'
import InstanceListPage from '../pages/instances/InstanceListPage.vue'
import LoginPage from '../pages/LoginPage.vue'
import RegisterPage from '../pages/RegisterPage.vue'
import { useAuthStore } from '../stores/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'home',
    redirect: '/dashboard',
  },
  {
    path: '/login',
    name: 'login',
    component: LoginPage,
    meta: {
      publicOnly: true,
    },
  },
  {
    path: '/register',
    name: 'register',
    component: RegisterPage,
    meta: {
      publicOnly: true,
    },
  },
  {
    path: '/',
    component: AppLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: 'dashboard',
        name: 'dashboard',
        component: DashboardPage,
        meta: {
          requiresAdmin: true,
          title: '仪表盘',
        },
      },
      {
        path: 'instances',
        name: 'instances',
        component: InstanceListPage,
        meta: {
          title: '实例列表',
        },
      },
      {
        path: 'instances/:id',
        name: 'instance-detail',
        component: () => import('../pages/instances/InstanceDetailPage.vue'),
        meta: {
          title: '实例详情',
        },
      },
      {
        path: 'targets',
        name: 'targets',
        component: () => import('../pages/targets/BackupTargetPage.vue'),
        meta: {
          requiresAdmin: true,
          title: '备份目标',
        },
      },
      {
        path: 'system',
        name: 'system-config',
        component: () => import('../pages/system/SystemConfigPage.vue'),
        meta: {
          requiresAdmin: true,
          title: '系统配置',
        },
      },
      {
        path: 'system/risks',
        name: 'system-risks',
        component: () => import('../pages/ComingSoonPage.vue'),
        meta: {
          requiresAdmin: true,
          title: '风险事件',
        },
      },
      {
        path: 'profile',
        name: 'profile',
        component: () => import('../pages/ComingSoonPage.vue'),
        meta: {
          title: '个人中心',
        },
      },
    ],
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  const authStore = useAuthStore()
  await authStore.ensureInitialized()

  const requiresAuth = to.matched.some((record) => record.meta.requiresAuth)
  const requiresAdmin = to.matched.some((record) => record.meta.requiresAdmin)
  const isPublicOnlyRoute = to.matched.some((record) => record.meta.publicOnly)

  if (isPublicOnlyRoute && authStore.isAuthenticated) {
    return authStore.defaultRoute
  }

  if (!requiresAuth) {
    return true
  }

  if (!authStore.isAuthenticated) {
    return '/login'
  }

  if (requiresAdmin && !authStore.isAdmin) {
    return '/instances'
  }

  return true
})