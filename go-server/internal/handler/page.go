// handler 包定义 HTTP 请求处理器
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rocuae/importsshkey/go-server/internal/repository"
)

// PageHandler 页面处理器
type PageHandler struct {
	userRepo repository.UserRepository
}

// NewPageHandler 创建页面处理器
// 参数：
//   - userRepo: 用户仓储
// 返回：
//   - *PageHandler: 处理器实例
func NewPageHandler(userRepo repository.UserRepository) *PageHandler {
	return &PageHandler{userRepo: userRepo}
}

// Stats 统计数据响应
type Stats struct {
	TotalUsers     int            `json:"total_users"`
	Version        string         `json:"version"`
	RecentActivity []ActivityItem `json:"recent_activity"`
}

// ActivityItem 活动记录
type ActivityItem struct {
	User    string `json:"user"`
	Action  string `json:"action"`
	Time    string `json:"time"`
	Source  string `json:"source"`
}

// Page 主页面
// @Summary 主页面
// @Description 返回 Web 管理界面
// @Tags page
// @Produce html
// @Router / [get]
func (h *PageHandler) Page(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, getHTML())
}

// StatsAPI 统计数据接口
// @Summary 获取统计数据
// @Description 返回用户数量和最近活动
// @Tags stats
// @Produce json
// @Success 200 {object} Stats
// @Router /stats [get]
func (h *PageHandler) StatsAPI(c *gin.Context) {
	users, err := h.userRepo.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	// 获取最近活动（取最后5个用户）
	recentActivity := make([]ActivityItem, 0)
	limit := 5
	if len(users) < limit {
		limit = len(users)
	}

	// 按更新时间倒序
	for i := len(users) - 1; i >= len(users)-limit && i >= 0; i-- {
		user := users[i]
		recentActivity = append(recentActivity, ActivityItem{
			User:   user.Username,
			Action: "update",
			Time:   user.UpdatedAt.Format(time.RFC3339),
			Source: user.Source,
		})
	}

	c.JSON(http.StatusOK, Stats{
		TotalUsers:     len(users),
		Version:        "v1.0",
		RecentActivity: recentActivity,
	})
}

func getHTML() string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>iskey-server</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; }
    .container { max-width: 800px; margin: 0 auto; padding: 20px; }
    .header { background: #2563eb; color: white; padding: 24px; border-radius: 8px; margin-bottom: 20px; }
    .header h1 { font-size: 24px; margin-bottom: 4px; }
    .header p { opacity: 0.9; }
    .status { display: flex; align-items: center; gap: 8px; margin-top: 12px; }
    .status-dot { width: 8px; height: 8px; background: #4ade80; border-radius: 50%; }
    .card { background: white; border-radius: 8px; padding: 20px; margin-bottom: 20px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
    .card h2 { font-size: 18px; margin-bottom: 16px; color: #1e40af; }
    .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 16px; }
    .stat { text-align: center; padding: 16px; background: #f0f9ff; border-radius: 8px; }
    .stat-value { font-size: 32px; font-weight: bold; color: #2563eb; }
    .stat-label { font-size: 14px; color: #6b7280; margin-top: 4px; }
    .activity-list { list-style: none; }
    .activity-item { padding: 12px 0; border-bottom: 1px solid #e5e7eb; display: flex; justify-content: space-between; align-items: center; }
    .activity-item:last-child { border-bottom: none; }
    .activity-user { font-weight: 500; }
    .activity-time { font-size: 14px; color: #6b7280; }
    .activity-action { font-size: 12px; padding: 2px 8px; border-radius: 4px; }
    .action-add { background: #dcfce7; color: #166534; }
    .action-update { background: #dbeafe; color: #1e40af; }
    .action-delete { background: #fee2e2; color: #991b1b; }
    .login-form { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
    .login-form input { flex: 1; min-width: 200px; padding: 10px 12px; border: 1px solid #d1d5db; border-radius: 6px; font-size: 14px; }
    .btn { padding: 10px 20px; border: none; border-radius: 6px; font-size: 14px; cursor: pointer; transition: background 0.2s; }
    .btn-primary { background: #2563eb; color: white; }
    .btn-primary:hover { background: #1d4ed8; }
    .btn-danger { background: #dc2626; color: white; }
    .btn-danger:hover { background: #b91c1c; }
    .btn-sm { padding: 6px 12px; font-size: 12px; }
    .admin-panel { display: none; }
    .admin-panel.active { display: block; }
    .query-section { display: none; }
    .query-section.active { display: block; }
    .query-result { margin-top: 12px; padding: 12px; background: #f3f4f6; border-radius: 6px; font-family: monospace; font-size: 13px; word-break: break-all; display: none; }
    .query-result.active { display: block; }
    .user-table { width: 100%; border-collapse: collapse; }
    .user-table th, .user-table td { padding: 12px; text-align: left; border-bottom: 1px solid #e5e7eb; }
    .user-table th { font-weight: 600; color: #6b7280; font-size: 14px; }
    .add-form { display: grid; gap: 12px; }
    .add-form input, .add-form textarea { padding: 10px 12px; border: 1px solid #d1d5db; border-radius: 6px; font-size: 14px; }
    .add-form textarea { min-height: 80px; resize: vertical; }
    .key-input-group { display: grid; gap: 8px; }
    .file-input-row { display: flex; align-items: center; gap: 8px; }
    .file-hint { font-size: 13px; color: #6b7280; }
    .btn-secondary { background: #6b7280; color: white; }
    .btn-secondary:hover { background: #4b5563; }
    .api-list { font-family: monospace; font-size: 14px; }
    .api-item { padding: 8px 0; display: flex; gap: 12px; }
    .api-method { font-weight: bold; min-width: 60px; }
    .api-path { color: #2563eb; }
    .api-auth { font-size: 12px; color: #6b7280; }
    .message { padding: 12px; border-radius: 6px; margin-bottom: 12px; display: none; }
    .message.success { display: block; background: #dcfce7; color: #166534; }
    .message.error { display: block; background: #fee2e2; color: #991b1b; }
    .logout-btn { background: none; border: 1px solid white; color: white; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 13px; }
    .logout-btn:hover { background: rgba(255,255,255,0.1); }
    .header-top { display: flex; justify-content: space-between; align-items: flex-start; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <div class="header-top">
        <div>
          <h1>iskey-server</h1>
          <p>SSH 公钥分发服务</p>
        </div>
        <button class="logout-btn" id="logoutBtn" style="display:none" onclick="logout()">退出登录</button>
      </div>
      <div class="status">
        <span class="status-dot"></span>
        <span>运行中</span>
      </div>
    </div>

    <div class="card">
      <h2>统计数据</h2>
      <div class="stats">
        <div class="stat">
          <div class="stat-value" id="totalUsers">-</div>
          <div class="stat-label">用户数</div>
        </div>
        <div class="stat">
          <div class="stat-value" id="version">-</div>
          <div class="stat-label">版本</div>
        </div>
      </div>
    </div>

    <div class="card">
      <h2>最近活动</h2>
      <ul class="activity-list" id="activityList">
        <li class="activity-item">加载中...</li>
      </ul>
    </div>

    <div class="card query-section" id="querySection">
      <h2>查询公钥</h2>
      <div class="login-form">
        <input type="text" id="queryUser" placeholder="输入用户名...">
        <button class="btn btn-primary" onclick="queryKey()">查询</button>
      </div>
      <div class="query-result" id="queryResult"></div>
    </div>

    <div class="card">
      <h2>管理面板</h2>
      <div id="loginPanel">
        <p style="margin-bottom: 12px; color: #6b7280;">输入 Admin Token 登录以管理公钥</p>
        <div class="login-form">
          <input type="password" id="tokenInput" placeholder="输入 Admin Token...">
          <button class="btn btn-primary" onclick="login()">登录</button>
        </div>
        <div class="message" id="loginMessage"></div>
      </div>

      <div class="admin-panel" id="adminPanel">
        <div class="message" id="adminMessage"></div>

        <h3 style="margin-bottom: 12px;">用户列表</h3>
        <table class="user-table">
          <thead>
            <tr>
              <th>用户名</th>
              <th>来源</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody id="userTableBody">
            <tr><td colspan="4">加载中...</td></tr>
          </tbody>
        </table>

        <h3 style="margin: 20px 0 12px;">添加用户</h3>
        <div class="add-form">
          <input type="text" id="addUsername" placeholder="用户名">
          <div class="key-input-group">
            <textarea id="addPublicKey" placeholder="SSH 公钥内容"></textarea>
            <div class="file-input-row">
              <input type="file" id="keyFile" accept=".pub" onchange="loadKeyFile(this)" style="display:none">
              <button class="btn btn-secondary" onclick="document.getElementById('keyFile').click()">选择文件</button>
              <span id="fileHint" class="file-hint"></span>
            </div>
          </div>
          <input type="text" id="addSource" placeholder="来源（可选）">
          <button class="btn btn-primary" onclick="addKey()">添加</button>
        </div>
      </div>
    </div>

    <div class="card">
      <h2>API 端点</h2>
      <div class="api-list">
        <div class="api-item">
          <span class="api-method">GET</span>
          <span class="api-path">/keys/:username</span>
          <span class="api-auth">需认证</span>
        </div>
        <div class="api-item">
          <span class="api-method">PUT</span>
          <span class="api-path">/keys/:username</span>
          <span class="api-auth">需认证</span>
        </div>
        <div class="api-item">
          <span class="api-method">DELETE</span>
          <span class="api-path">/keys/:username</span>
          <span class="api-auth">需认证</span>
        </div>
        <div class="api-item">
          <span class="api-method">GET</span>
          <span class="api-path">/keys</span>
          <span class="api-auth">需认证</span>
        </div>
        <div class="api-item">
          <span class="api-method">GET</span>
          <span class="api-path">/health</span>
          <span class="api-auth">公开</span>
        </div>
        <div class="api-item">
          <span class="api-method">GET</span>
          <span class="api-path">/stats</span>
          <span class="api-auth">公开</span>
        </div>
      </div>
    </div>
  </div>

  <script>
    let adminToken = '';

    window.onload = function() {
      loadStats();
      const savedToken = sessionStorage.getItem('admin_token');
      if (savedToken) {
        adminToken = savedToken;
        verifyToken();
      }
    };

    async function loadStats() {
      try {
        const resp = await fetch('/stats');
        const data = await resp.json();
        document.getElementById('totalUsers').textContent = data.total_users;
        document.getElementById('version').textContent = data.version || 'v1';

        const activityList = document.getElementById('activityList');
        if (data.recent_activity && data.recent_activity.length > 0) {
          activityList.innerHTML = data.recent_activity.map(item => {
            const time = new Date(item.time).toLocaleString('zh-CN');
            const actionClass = item.action === 'add' ? 'action-add' : item.action === 'delete' ? 'action-delete' : 'action-update';
            const actionText = item.action === 'add' ? '添加' : item.action === 'delete' ? '删除' : '更新';
            return '<li class="activity-item">' +
              '<div><span class="activity-user">' + item.user + '</span> <span class="activity-action ' + actionClass + '">' + actionText + '公钥</span></div>' +
              '<span class="activity-time">' + time + '</span>' +
            '</li>';
          }).join('');
        } else {
          activityList.innerHTML = '<li class="activity-item">暂无活动记录</li>';
        }
      } catch (e) {
        console.error('Failed to load stats:', e);
      }
    }

    async function login() {
      const token = document.getElementById('tokenInput').value.trim();
      if (!token) return;
      adminToken = token;
      await verifyToken();
    }

    async function verifyToken() {
      const msg = document.getElementById('loginMessage');
      try {
        const resp = await fetch('/keys', {
          headers: { 'Authorization': 'Bearer ' + adminToken }
        });

        if (resp.ok) {
          sessionStorage.setItem('admin_token', adminToken);
          document.getElementById('loginPanel').style.display = 'none';
          document.getElementById('adminPanel').classList.add('active');
          document.getElementById('querySection').classList.add('active');
          document.getElementById('logoutBtn').style.display = 'block';
          msg.className = 'message';
          msg.style.display = 'none';
          loadUserList();
        } else {
          throw new Error('Token 无效');
        }
      } catch (e) {
        msg.className = 'message error';
        msg.textContent = '认证失败: ' + e.message;
        msg.style.display = 'block';
        adminToken = '';
        sessionStorage.removeItem('admin_token');
      }
    }

    function logout() {
      adminToken = '';
      sessionStorage.removeItem('admin_token');
      document.getElementById('loginPanel').style.display = 'block';
      document.getElementById('adminPanel').classList.remove('active');
      document.getElementById('querySection').classList.remove('active');
      document.getElementById('logoutBtn').style.display = 'none';
      document.getElementById('tokenInput').value = '';
    }

    async function loadUserList() {
      try {
        const resp = await fetch('/keys', {
          headers: { 'Authorization': 'Bearer ' + adminToken }
        });
        const data = await resp.json();
        const tbody = document.getElementById('userTableBody');

        if (data.users && data.users.length > 0) {
          const rows = await Promise.all(data.users.map(async (username) => {
            try {
              const metaResp = await fetch('/keys/' + username + '/metadata', {
                headers: { 'Authorization': 'Bearer ' + adminToken }
              });
              const metaData = await metaResp.json();
              const source = metaData.metadata?.source || '-';
              const time = metaData.metadata?.updated_at ? new Date(metaData.metadata.updated_at).toLocaleString('zh-CN') : '-';
              return '<tr>' +
                '<td>' + username + '</td>' +
                '<td>' + source + '</td>' +
                '<td>' + time + '</td>' +
                '<td><button class="btn btn-danger btn-sm" onclick="deleteKey(\\'' + username + '\\')">删除</button></td>' +
              '</tr>';
            } catch {
              return '<tr>' +
                '<td>' + username + '</td>' +
                '<td>-</td>' +
                '<td>-</td>' +
                '<td><button class="btn btn-danger btn-sm" onclick="deleteKey(\\'' + username + '\\')">删除</button></td>' +
              '</tr>';
            }
          }));
          tbody.innerHTML = rows.join('');
        } else {
          tbody.innerHTML = '<tr><td colspan="4">暂无用户</td></tr>';
        }
      } catch (e) {
        console.error('Failed to load users:', e);
      }
    }

    async function queryKey() {
      const username = document.getElementById('queryUser').value.trim();
      if (!username) return;

      const result = document.getElementById('queryResult');
      try {
        const resp = await fetch('/keys/' + username, {
          headers: { 'Authorization': 'Bearer ' + adminToken }
        });

        if (resp.ok) {
          const key = await resp.text();
          result.textContent = key;
          result.className = 'query-result active';
        } else {
          const err = await resp.json();
          result.textContent = '错误: ' + (err.error || '用户不存在');
          result.className = 'query-result active';
          result.style.color = '#dc2626';
        }
      } catch (e) {
        result.textContent = '查询失败: ' + e.message;
        result.className = 'query-result active';
        result.style.color = '#dc2626';
      }
    }

    async function addKey() {
      const username = document.getElementById('addUsername').value.trim();
      const publicKey = document.getElementById('addPublicKey').value.trim();
      const source = document.getElementById('addSource').value.trim();

      if (!username || !publicKey) {
        showMessage('adminMessage', '用户名和公钥不能为空', 'error');
        return;
      }

      try {
        const resp = await fetch('/keys/' + username, {
          method: 'PUT',
          headers: {
            'Authorization': 'Bearer ' + adminToken,
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({ public_key: publicKey, source: source })
        });

        const data = await resp.json();
        if (data.success) {
          showMessage('adminMessage', '添加成功', 'success');
          document.getElementById('addUsername').value = '';
          document.getElementById('addPublicKey').value = '';
          document.getElementById('addSource').value = '';
          loadUserList();
          loadStats();
        } else {
          showMessage('adminMessage', '添加失败: ' + data.error, 'error');
        }
      } catch (e) {
        showMessage('adminMessage', '添加失败: ' + e.message, 'error');
      }
    }

    async function deleteKey(username) {
      if (!confirm('确定删除用户 ' + username + ' 的公钥？')) return;

      try {
        const resp = await fetch('/keys/' + username, {
          method: 'DELETE',
          headers: { 'Authorization': 'Bearer ' + adminToken }
        });

        const data = await resp.json();
        if (data.success) {
          showMessage('adminMessage', '删除成功', 'success');
          loadUserList();
          loadStats();
        } else {
          showMessage('adminMessage', '删除失败: ' + data.error, 'error');
        }
      } catch (e) {
        showMessage('adminMessage', '删除失败: ' + e.message, 'error');
      }
    }

    function showMessage(elementId, text, type) {
      const msg = document.getElementById(elementId);
      msg.textContent = text;
      msg.className = 'message ' + type;
      msg.style.display = 'block';
      setTimeout(() => { msg.style.display = 'none'; }, 3000);
    }

    document.getElementById('queryUser').addEventListener('keypress', function(e) {
      if (e.key === 'Enter') queryKey();
    });

    document.getElementById('tokenInput').addEventListener('keypress', function(e) {
      if (e.key === 'Enter') login();
    });

    // 文件选择读取公钥
    function loadKeyFile(input) {
      const file = input.files[0];
      if (!file) return;
      
      const reader = new FileReader();
      reader.onload = function(e) {
        document.getElementById('addPublicKey').value = e.target.result.trim();
        document.getElementById('fileHint').textContent = '✓ ' + file.name;
      };
      reader.onerror = function() {
        document.getElementById('fileHint').textContent = '✗ 读取失败';
      };
      reader.readAsText(file);
    }
  </script>
</body>
</html>`
}
