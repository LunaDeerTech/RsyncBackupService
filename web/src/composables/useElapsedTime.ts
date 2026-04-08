import { ref, onUnmounted, watch, type Ref } from 'vue'

function formatDuration(ms: number): string {
  if (ms < 0) return '00:00:00'
  const totalSeconds = Math.floor(ms / 1000)
  const h = Math.floor(totalSeconds / 3600)
  const m = Math.floor((totalSeconds % 3600) / 60)
  const s = totalSeconds % 60
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

export function useElapsedTime(startTime: Ref<string | null>): Ref<string> {
  const elapsed = ref('00:00:00')
  let timer: ReturnType<typeof setInterval> | null = null

  function update() {
    if (!startTime.value) {
      elapsed.value = '00:00:00'
      return
    }
    const diff = Date.now() - new Date(startTime.value).getTime()
    elapsed.value = formatDuration(diff)
  }

  function start() {
    stop()
    if (startTime.value) {
      update()
      timer = setInterval(update, 1000)
    }
  }

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  watch(startTime, (val) => {
    if (val) start()
    else stop()
  }, { immediate: true })

  onUnmounted(stop)

  return elapsed
}
