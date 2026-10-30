/**
 * 桌面端注册/改资料时把打包内置头像的绝对路径写进了 ptmj_user.avatar，
 * 形如 file:///D:/Max/pezmax/resources/app.asar/out/renderer/static/jpg/default_avatar-BE-pmgET.jpg。
 * 该路径只对写入它的那台机器有效：管理后台既加载不了，也没有展示价值，
 * 所以默认头像在列表里统一显示成 default_avatar 标记。
 */
export const DEFAULT_AVATAR_LABEL = 'default_avatar';

// 默认头像文件名：default、default.png、default_avatar、default_avatar-BE-pmgET.jpg 等。
// 先取路径最后一段再去掉扩展名与查询串，才能同时匹配 file:// 绝对路径和裸文件名。
const DEFAULT_AVATAR_STEM = /^default(?:_avatar)?(?:[-_.].*)?$/i;

/** 列表展示用头像文案：默认头像收敛为 default_avatar，空值保持 -。 */
export function avatarLabel(avatar?: null | string): string {
  const value = (avatar ?? '').trim();
  if (!value || value === 'null') {
    return '-';
  }
  const fileName = (value.split(/[\\/]/).pop() ?? '').split(/[?#]/)[0] ?? '';
  const stem = fileName.replace(/\.[^.]*$/, '');
  return DEFAULT_AVATAR_STEM.test(stem) ? DEFAULT_AVATAR_LABEL : value;
}
