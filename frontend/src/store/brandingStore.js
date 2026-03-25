import { create } from 'zustand'
import axios from 'axios'
import api from '../services/api'

// Apply primary color as CSS variable
const applyPrimaryColor = (color) => {
  if (!color) return
  document.documentElement.style.setProperty('--color-primary', color)
  // Generate color variants (lighter/darker)
  const hex = color.replace('#', '')
  const r = parseInt(hex.substr(0, 2), 16)
  const g = parseInt(hex.substr(2, 2), 16)
  const b = parseInt(hex.substr(4, 2), 16)
  // Light variant (for backgrounds)
  document.documentElement.style.setProperty('--color-primary-light', `rgba(${r}, ${g}, ${b}, 0.1)`)
  // Dark variant (for hover states)
  document.documentElement.style.setProperty('--color-primary-dark', `rgb(${Math.max(0, r - 20)}, ${Math.max(0, g - 20)}, ${Math.max(0, b - 20)})`)
}

// Apply favicon
const applyFavicon = (faviconUrl) => {
  if (!faviconUrl) return
  let link = document.querySelector("link[rel~='icon']")
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.href = faviconUrl
}

export const useBrandingStore = create((set, get) => ({
  companyName: '',
  companyLogo: '',
  primaryColor: '#2563eb',
  loginBackground: '',
  favicon: '',
  footerText: '',
  loginTagline: 'High Performance ISP Management Solution',
  showLoginFeatures: true,
  loginFeature1Title: 'PPPoE Management',
  loginFeature1Desc: 'Complete subscriber and session management with real-time monitoring',
  loginFeature2Title: 'Bandwidth Control',
  loginFeature2Desc: 'FUP quotas, time-based speed control, and usage monitoring',
  loginFeature3Title: 'MikroTik Integration',
  loginFeature3Desc: 'Seamless RADIUS and API integration with MikroTik routers',
  loaded: false,
  loading: false,

  fetchBranding: async () => {
    if (get().loading) return
    set({ loading: true })
    try {
      const response = await api.get('/branding').catch(() => axios.get('/api/branding'))
      if (response.data.success) {
        const data = response.data.data
        const name = data.company_name || ''
        const logo = data.company_logo || ''
        const color = data.primary_color || '#2563eb'
        const background = data.login_background || ''
        const fav = data.favicon || ''
        const footer = data.footer_text || ''

        set({
          companyName: name,
          companyLogo: logo,
          primaryColor: color,
          loginBackground: background,
          favicon: fav,
          footerText: footer,
          loginTagline: data.login_tagline || 'High Performance ISP Management Solution',
          showLoginFeatures: data.show_login_features !== 'false',
          loginFeature1Title: data.login_feature_1_title || 'PPPoE Management',
          loginFeature1Desc: data.login_feature_1_desc || 'Complete subscriber and session management with real-time monitoring',
          loginFeature2Title: data.login_feature_2_title || 'Bandwidth Control',
          loginFeature2Desc: data.login_feature_2_desc || 'FUP quotas, time-based speed control, and usage monitoring',
          loginFeature3Title: data.login_feature_3_title || 'MikroTik Integration',
          loginFeature3Desc: data.login_feature_3_desc || 'Seamless RADIUS and API integration with MikroTik routers',
          loaded: true,
        })

        // Update browser title dynamically
        const title = name ? `${name} - ISP Management` : 'ISP Management System'
        document.title = title

        // Apply primary color
        applyPrimaryColor(color)

        // Apply favicon
        if (fav) {
          applyFavicon(fav)
        }
      }
    } catch (error) {
      console.error('Failed to fetch branding:', error)
    } finally {
      set({ loading: false })
    }
  },

  updateBranding: (data) => {
    const state = get()
    const updates = {
      companyName: data.company_name !== undefined ? data.company_name : state.companyName,
      companyLogo: data.company_logo !== undefined ? data.company_logo : state.companyLogo,
      primaryColor: data.primary_color !== undefined ? data.primary_color : state.primaryColor,
      loginBackground: data.login_background !== undefined ? data.login_background : state.loginBackground,
      favicon: data.favicon !== undefined ? data.favicon : state.favicon,
      footerText: data.footer_text !== undefined ? data.footer_text : state.footerText,
      loginTagline: data.login_tagline !== undefined ? data.login_tagline : state.loginTagline,
      showLoginFeatures: data.show_login_features !== undefined ? data.show_login_features : state.showLoginFeatures,
      loginFeature1Title: data.login_feature_1_title !== undefined ? data.login_feature_1_title : state.loginFeature1Title,
      loginFeature1Desc: data.login_feature_1_desc !== undefined ? data.login_feature_1_desc : state.loginFeature1Desc,
      loginFeature2Title: data.login_feature_2_title !== undefined ? data.login_feature_2_title : state.loginFeature2Title,
      loginFeature2Desc: data.login_feature_2_desc !== undefined ? data.login_feature_2_desc : state.loginFeature2Desc,
      loginFeature3Title: data.login_feature_3_title !== undefined ? data.login_feature_3_title : state.loginFeature3Title,
      loginFeature3Desc: data.login_feature_3_desc !== undefined ? data.login_feature_3_desc : state.loginFeature3Desc,
    }

    set(updates)

    const name = updates.companyName
    const color = updates.primaryColor
    const fav = updates.favicon

    // Update browser title dynamically
    const title = name ? `${name} - ISP Management` : 'ISP Management System'
    document.title = title

    // Apply primary color
    applyPrimaryColor(color)

    // Apply favicon
    if (fav) {
      applyFavicon(fav)
    }
  },
}))
