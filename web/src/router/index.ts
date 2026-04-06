import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import DashboardPage from '../pages/DashboardPage.vue'
import InstancesPage from '../pages/InstancesPage.vue'
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
    path: '/dashboard',
    name: 'dashboard',
    component: DashboardPage,
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
    },
  },
  {
    path: '/instances',
    name: 'instances',
    component: InstancesPage,
    meta: {
      requiresAuth: true,
    },
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