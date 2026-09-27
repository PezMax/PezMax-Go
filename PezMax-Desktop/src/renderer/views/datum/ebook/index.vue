<template>
  <div class="ebook-page">
    <div class="ebook-hero">
      <svg-icon icon-class="education" class="ebook-hero-icon" />
      <div>
        <h3>电子书</h3>
        <p>电子书阅读与管理，功能开发中。</p>
      </div>
    </div>

    <div class="ebook-placeholder">
      <div class="empty-state">
        <img class="empty-img" src="@/assets/images/ebook.png" alt="" draggable="false" />
        <div class="empty-title">功能建设中</div>
        <div class="empty-desc">电子书栏目正在筹备中，敬请期待</div>
      </div>
    </div>
  </div>
</template>

<script setup>
defineOptions({ name: 'Ebook' })
</script>

<style scoped lang="scss">
/* 与 RankView 一致：作为首页 IDE 视图嵌入时填满内容区 */
.ebook-page {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: hidden;
  background: var(--ide-panel-bg, #fff);
  border: 1px solid var(--ide-border, #ebeef5);
  border-radius: 12px;
  padding: 16px;
}
.ebook-hero {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border-radius: 12px;
  border: 1px solid rgba(var(--ide-accent-rgb, 64, 158, 255), 0.18);
  background: linear-gradient(
      135deg,
      rgba(var(--ide-accent-rgb, 64, 158, 255), 0.14),
      rgba(var(--ide-accent-rgb, 64, 158, 255), 0.04)
  );
}
.ebook-hero-icon {
  font-size: 22px;
  color: var(--ide-accent, #409eff);
}
.ebook-hero h3 {
  margin: 0;
  font-size: 18px;
  letter-spacing: 0.3px;
  color: var(--ide-text-active, #303133);
}
.ebook-hero p {
  margin: 2px 0 0;
  color: var(--ide-text-light, #909399);
  font-size: 13px;
}
.ebook-placeholder {
  flex: 1;
  min-height: 320px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--ide-border, #dcdfe6);
  border-radius: 12px;
  background: color-mix(in srgb, var(--ide-editor-bg, #fff) 94%, transparent);
}
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  color: var(--ide-text-light, #909399);
}
.empty-img {
  width: 56px;
  height: 56px;
  object-fit: contain;
  margin-bottom: 8px;
  opacity: 0.75;
  user-select: none;
}
.empty-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--ide-text-active, #303133);
}
.empty-desc {
  font-size: 13px;
}
</style>
