import { ref, watch, onUnmounted, type Ref } from 'vue'

/**
 * Animate a number from 0 to target over `duration` ms using easeOutCubic.
 * Returns a reactive ref that ticks toward the latest target.
 */
export function useCountUp(
  target: Ref<number>,
  duration = 600,
): Ref<number> {
  const display = ref(0)
  let raf: number | null = null
  let startVal = 0
  let startTime = 0
  let endVal = 0

  function easeOutCubic(t: number): number {
    return 1 - Math.pow(1 - t, 3)
  }

  function tick() {
    const elapsed = Date.now() - startTime
    const progress = Math.min(elapsed / duration, 1)
    display.value = Math.round(startVal + (endVal - startVal) * easeOutCubic(progress))
    if (progress < 1) {
      raf = requestAnimationFrame(tick)
    } else {
      display.value = endVal
      raf = null
    }
  }

  watch(target, (newVal) => {
    if (raf != null) cancelAnimationFrame(raf)
    startVal = display.value
    endVal = newVal
    startTime = Date.now()
    raf = requestAnimationFrame(tick)
  }, { immediate: true })

  onUnmounted(() => {
    if (raf != null) cancelAnimationFrame(raf)
  })

  return display
}
