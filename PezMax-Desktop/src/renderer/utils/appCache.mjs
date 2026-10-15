const localCacheKeys = ['ptmj_file_tree_cache', 'ptmj_upload_rank_cache']
const sessionCacheKeys = ['sessionObj']

export function clearApplicationCaches(local, session) {
  for (const key of localCacheKeys) local?.removeItem(key)
  for (const key of sessionCacheKeys) session?.removeItem(key)
}
