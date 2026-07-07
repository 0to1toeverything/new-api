import { API } from './index';

let cachedDeptMap = null; // id -> { id, name, parent_id }
let cachedDeptPathMap = null; // id -> "path string"

export async function fetchDepartmentData() {
  if (cachedDeptMap) return { map: cachedDeptMap, pathMap: cachedDeptPathMap };
  try {
    const res = await API.get('/api/department/names');
    if (res.data.success && Array.isArray(res.data.data)) {
      const map = {};
      res.data.data.forEach((d) => { map[d.id] = d; });
      cachedDeptMap = map;

      // Build path map: for each dept, walk up parent chain
      const pathMap = {};
      for (const id in map) {
        pathMap[id] = buildDeptPath(parseInt(id), map);
      }
      cachedDeptPathMap = pathMap;
      return { map, pathMap };
    }
  } catch (e) { /* ignore */ }
  return { map: {}, pathMap: {} };
}

function buildDeptPath(deptId, map, maxDepth = 10) {
  const parts = [];
  let current = deptId;
  for (let i = 0; i < maxDepth; i++) {
    const dept = map[current];
    if (!dept) break;
    parts.unshift(dept.name);
    if (!dept.parent_id || dept.parent_id === 0) break;
    current = dept.parent_id;
  }
  return parts.join(' / ');
}

export function getDepartmentName(deptId) {
  if (!cachedDeptMap || !deptId) return null;
  const dept = cachedDeptMap[deptId];
  return dept ? dept.name : null;
}

export function getDepartmentPath(deptId) {
  if (!cachedDeptPathMap || !deptId) return null;
  return cachedDeptPathMap[deptId] || null;
}
