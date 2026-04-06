import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import ComingSoonPage from '../pages/ComingSoonPage.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'home',
    component: ComingSoonPage,
  },
  // { path: '/login', name: 'login', component: LoginPage },
  // { path: '/register', name: 'register', component: RegisterPage },
  // { path: '/dashboard', name: 'dashboard', component: DashboardPage },
  // { path: '/instances', name: 'instances', component: InstancesPage },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})