import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { listTasks, getTask, cancelTask, type TaskItem } from '../api/tasks'
import { useToastStore } from './toast'

export const useTaskStore = defineStore('task', () => {
  const activeTasks = ref<TaskItem[]>([])
  let globalTimer: ReturnType<typeof setInterval> | null = null
  const taskWatchers = new Map<number, ReturnType<typeof setInterval>>()

  const runningCount = computed(() =>
    activeTasks.value.filter(t => t.status === 'running').length,
  )

  async function fetchActiveTasks() {
    try {
      const res = await listTasks()
      activeTasks.value = res.items ?? []
    } catch {
      // silent
    }
  }

  function startPolling() {
    stopPolling()
    fetchActiveTasks()
    globalTimer = setInterval(fetchActiveTasks, 10000)
  }

  function stopPolling() {
    if (globalTimer) {
      clearInterval(globalTimer)
      globalTimer = null
    }
    // Stop all task-level watchers too
    for (const [id, timer] of taskWatchers) {
      clearInterval(timer)
      taskWatchers.delete(id)
    }
    activeTasks.value = []
  }

  function getTasksByInstance(instanceId: number): TaskItem[] {
    return activeTasks.value.filter(t => t.instance_id === instanceId)
  }

  function watchTask(taskId: number, onUpdate: (task: TaskItem) => void): () => void {
    // Stop existing watcher for same task
    if (taskWatchers.has(taskId)) {
      clearInterval(taskWatchers.get(taskId)!)
      taskWatchers.delete(taskId)
    }

    const toast = useToastStore()

    async function poll() {
      try {
        const task = await getTask(taskId)
        onUpdate(task)

        // Update in activeTasks list too
        const idx = activeTasks.value.findIndex(t => t.id === taskId)
        if (idx >= 0) {
          activeTasks.value[idx] = task
        }

        if (task.status === 'success' || task.status === 'failed' || task.status === 'cancelled') {
          stopWatcher()
          if (task.status === 'success') {
            toast.success(`任务 #${taskId} 完成`)
          } else if (task.status === 'failed') {
            toast.error(`任务 #${taskId} 失败: ${task.error_message || '未知错误'}`)
          } else {
            toast.info(`任务 #${taskId} 已取消`)
          }
          // Refresh global list
          fetchActiveTasks()
        }
      } catch {
        // silent
      }
    }

    const timer = setInterval(poll, 2000)
    taskWatchers.set(taskId, timer)
    // Immediate first poll
    poll()

    function stopWatcher() {
      const t = taskWatchers.get(taskId)
      if (t) {
        clearInterval(t)
        taskWatchers.delete(taskId)
      }
    }

    return stopWatcher
  }

  async function doCancelTask(taskId: number) {
    const toast = useToastStore()
    try {
      await cancelTask(taskId)
      toast.success('已发送取消请求')
      fetchActiveTasks()
    } catch (e: any) {
      toast.error(e?.message ?? '取消任务失败')
    }
  }

  return {
    activeTasks,
    runningCount,
    fetchActiveTasks,
    startPolling,
    stopPolling,
    getTasksByInstance,
    watchTask,
    doCancelTask,
  }
})
