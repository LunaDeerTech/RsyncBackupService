<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ChevronDown, Check } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    modelValue?: string | number
    options: { label: string; value: string | number }[]
    placeholder?: string
    disabled?: boolean
  }>(),
  {
    modelValue: '',
    placeholder: '请选择',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string | number]
}>()

const open = ref(false)
const wrapperRef = ref<HTMLElement | null>(null)

const selectedLabel = computed(() => {
  const opt = props.options.find((o) => String(o.value) === String(props.modelValue))
  return opt?.label ?? ''
})

function toggle() {
  if (!props.disabled) open.value = !open.value
}

function select(value: string | number) {
  emit('update:modelValue', value)
  open.value = false
}

function onClickOutside(e: MouseEvent) {
  if (wrapperRef.value && !wrapperRef.value.contains(e.target as Node)) {
    open.value = false
  }
}

onMounted(() => document.addEventListener('mousedown', onClickOutside))
onBeforeUnmount(() => document.removeEventListener('mousedown', onClickOutside))
</script>

<template>
  <div ref="wrapperRef" class="app-select" :class="{ 'app-select--open': open, 'app-select--disabled': disabled }">
    <button type="button" class="app-select__trigger" @click="toggle">
      <span :class="selectedLabel ? 'app-select__value' : 'app-select__placeholder'">
        {{ selectedLabel || placeholder }}
      </span>
      <ChevronDown :size="16" class="app-select__arrow" />
    </button>
    <Transition name="dropdown">
      <ul v-if="open" class="app-select__dropdown">
        <li
          v-for="opt in options"
          :key="opt.value"
          class="app-select__option"
          :class="{ 'app-select__option--selected': String(opt.value) === String(modelValue) }"
          @click="select(opt.value)"
        >
          <span>{{ opt.label }}</span>
          <Check v-if="String(opt.value) === String(modelValue)" :size="14" class="app-select__check" />
        </li>
      </ul>
    </Transition>
  </div>
</template>

<style scoped>
.app-select {
  position: relative;
  width: 100%;
}
.app-select__trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 8px 12px;
  font-size: 14px;
  line-height: 20px;
  color: var(--text-primary);
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  outline: none;
  text-align: left;
}
.app-select__trigger:focus {
  border-color: var(--border-focus);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary-500) 25%, transparent);
}
.app-select--open .app-select__trigger {
  border-color: var(--border-focus);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary-500) 25%, transparent);
}
.app-select--disabled .app-select__trigger {
  opacity: 0.55;
  cursor: not-allowed;
}
.app-select__value {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.app-select__placeholder {
  flex: 1;
  color: var(--text-muted);
}
.app-select__arrow {
  flex-shrink: 0;
  color: var(--text-muted);
  transition: transform var(--transition-fast);
}
.app-select--open .app-select__arrow {
  transform: rotate(180deg);
}
.app-select__dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  z-index: 100;
  max-height: 220px;
  overflow-y: auto;
}
.app-select__option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  font-size: 14px;
  color: var(--text-primary);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: background var(--transition-fast);
}
.app-select__option:hover {
  background: var(--surface-sunken);
}
.app-select__option--selected {
  color: var(--primary-500);
  font-weight: 500;
}
.app-select__check {
  color: var(--primary-500);
  flex-shrink: 0;
}

/* Dropdown transition */
.dropdown-enter-active { transition: opacity 150ms ease, transform 150ms ease; }
.dropdown-leave-active { transition: opacity 100ms ease, transform 100ms ease; }
.dropdown-enter-from { opacity: 0; transform: translateY(-4px); }
.dropdown-leave-to { opacity: 0; transform: translateY(-4px); }
</style>
