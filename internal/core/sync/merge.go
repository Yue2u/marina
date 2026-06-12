package sync

import "github.com/Yue2u/marina/internal/core"

// MergeHosts сливает remote в local: побеждает более свежий UpdatedAt.
// Tombstones (DeletedAt != nil) участвуют в сравнении — удаление тоже "выигрывает" по времени.
func MergeHosts(local, remote []core.Host) []core.Host {
	byID := make(map[string]core.Host, len(local))
	for _, h := range local {
		byID[h.ID] = h
	}
	for _, r := range remote {
		cur, exists := byID[r.ID]
		if !exists || r.UpdatedAt.After(cur.UpdatedAt) {
			byID[r.ID] = r
		}
	}
	result := make([]core.Host, 0, len(byID))
	for _, h := range byID {
		result = append(result, h)
	}
	return result
}

// MergeFolders сливает remote в local — папки не имеют UpdatedAt, поэтому
// remote побеждает при конфликте (папки обычно не редактируются часто).
func MergeFolders(local, remote []core.Folder) []core.Folder {
	byID := make(map[string]core.Folder, len(local))
	for _, f := range local {
		byID[f.ID] = f
	}
	for _, r := range remote {
		byID[r.ID] = r
	}
	result := make([]core.Folder, 0, len(byID))
	for _, f := range byID {
		result = append(result, f)
	}
	return result
}
