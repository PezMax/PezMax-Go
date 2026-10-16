<template>
  <div id="app">
    <!-- 全局统一的标题栏，自适应 Win/Mac，内置用户信息和通知 -->
    <TitleHeader />
    <div class="app-main-content">
      <RouterView v-slot="{ Component }">
        <!-- 保留当前会话的工作区；登录态变化时销毁缓存，避免跨账号复用。 -->
        <KeepAlive :key="userStore.token" include="HomeWorkspace">
          <component :is="Component" v-if="userStore.token || isPtmjAuthRoute(route.path)" />
        </KeepAlive>
      </RouterView>
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, watch, ref } from 'vue'
import { useRoute } from 'vue-router'
import TitleHeader from '@/components/TitleHeader/index.vue'
import { applyIdeAppearanceFromSettings, applyIdeThemeState, teardownIdeAppearanceMediaListener } from '@/utils/ideAppearance'
import { isPtmjAuthRoute } from '@/constants/ptmjAuth'
import { handleSessionExpired } from '@/utils/request'
import useUserStore from '@/store/modules/user'

const route = useRoute()
const userStore = useUserStore()
const savedDarkMode = ref(null)
let removeSessionExpiredListener = null

onMounted(async () => {
  removeSessionExpiredListener = window.electronAPI?.onSessionExpired?.((message) => {
    handleSessionExpired(message).catch(() => {})
  })
  await applyIdeAppearanceFromSettings()
  savedDarkMode.value = document.documentElement.classList.contains('dark')
  if (isPtmjAuthRoute(route.path)) {
    applyIdeThemeState(false)
  }
})

watch(() => route.path, (newPath, oldPath) => {
  const enteringAuth = isPtmjAuthRoute(newPath)
  const leavingAuth = isPtmjAuthRoute(oldPath)
  if (enteringAuth && !leavingAuth) {
    savedDarkMode.value = document.documentElement.classList.contains('dark')
    applyIdeThemeState(false)
  } else if (!enteringAuth && leavingAuth) {
    if (savedDarkMode.value) {
      applyIdeThemeState(true)
    }
  }
})

onUnmounted(() => {
  removeSessionExpiredListener?.()
  teardownIdeAppearanceMediaListener()
})
</script>

<style>
#app {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}
.app-main-content {
  flex: 1;
  overflow: auto;
  position: relative;
}
.app-main-content::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}
.app-main-content::-webkit-scrollbar-thumb {
  background-color: rgba(148, 163, 184, 0.4);
  border-radius: 3px;
}
</style>
