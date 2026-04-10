import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getStoredListViewModes, setStoredListViewModes, type StoredListViewMode } from '../utils/storage'

export type ListViewMode = StoredListViewMode
export const SHARED_LIST_VIEW_PREFERENCE_KEY = 'shared-list-view'

export const useListViewPreferenceStore = defineStore('list-view-preference', () => {
  const viewModes = ref<Record<string, ListViewMode>>(getStoredListViewModes())

  function getViewMode(key: string) {
    return viewModes.value[key]
  }

  function setViewMode(key: string, mode: ListViewMode) {
    if (viewModes.value[key] === mode) {
      return
    }

    viewModes.value = {
      ...viewModes.value,
      [key]: mode,
    }
    setStoredListViewModes(viewModes.value)
  }

  function initializeViewMode(key: string, fallbackMode: ListViewMode) {
    const storedMode = getViewMode(key)
    if (storedMode) {
      return storedMode
    }

    setViewMode(key, fallbackMode)
    return fallbackMode
  }

  return {
    viewModes,
    getViewMode,
    setViewMode,
    initializeViewMode,
  }
})