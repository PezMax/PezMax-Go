<template>
    <AConfigProvider :theme="themeConfig" :locale="zhCN">
        <slot />
    </AConfigProvider>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ConfigProvider as AConfigProvider, theme } from 'ant-design-vue'
import zhCN from 'ant-design-vue/es/locale/zh_CN'

const dark = ref(false)
const accent = ref('#409eff')
let observer
const syncTheme = () => {
    dark.value = document.documentElement.classList.contains('dark')
    accent.value =
        getComputedStyle(document.documentElement).getPropertyValue('--ide-accent').trim() ||
        '#409eff'
}
const themeConfig = computed(() => ({
    algorithm: dark.value ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
        colorPrimary: accent.value,
        borderRadius: 10,
        fontSize: 14,
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", "Microsoft YaHei", sans-serif',
        ...(dark.value
            ? { colorBgContainer: '#1e293b', colorBgElevated: '#1e293b', colorBorder: '#334155' }
            : {})
    }
}))
onMounted(() => {
    syncTheme()
    observer = new MutationObserver(syncTheme)
    observer.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ['class', 'style']
    })
})
onUnmounted(() => observer?.disconnect())
</script>
