import { create } from 'zustand'

// Get initial theme from localStorage or default to light
const getInitialTheme = () => {
  try {
    const saved = localStorage.getItem('theme')
    if (saved === 'dark') {
      document.documentElement.classList.add('dark')
      return 'dark'
    }
  } catch (e) {
    console.error('Failed to load theme:', e)
  }
  document.documentElement.classList.remove('dark')
  return 'light'
}

export const useThemeStore = create((set) => ({
  theme: getInitialTheme(),

  toggleTheme: () => set((state) => {
    const newTheme = state.theme === 'light' ? 'dark' : 'light'

    // Update DOM
    if (newTheme === 'dark') {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }

    // Save to localStorage
    try {
      localStorage.setItem('theme', newTheme)
    } catch (e) {
      console.error('Failed to save theme:', e)
    }

    return { theme: newTheme }
  }),

  isDark: () => {
    const state = useThemeStore.getState()
    return state.theme === 'dark'
  }
}))
