package kadmin

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/generated"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/codegen"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/files"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/jobs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loadrank"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loginlogs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/monitor"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/notifications"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
)

const (
	defaultHomePath = "/dashboard/workspace"
)

type Store struct {
	conn           db.Connection
	configMu       sync.Mutex
	menuMutationMu sync.Mutex
	auth           *authService
	security       *securityService
	audit          *businessAuditRecorder
	jobs           *jobs.Manager
	loginLogs      *loginlogs.Manager
	monitor        *monitor.Manager
	loadRank       *loadrank.Sampler
}

type Runtime struct {
	audit     *businessAuditRecorder
	jobs      *jobs.Manager
	loginLogs *loginlogs.Manager
	monitor   *monitor.Manager
	loadRank  *loadrank.Sampler
}

func (r *Runtime) Close() {
	if r == nil {
		return
	}
	if r.audit != nil {
		r.audit.Close()
	}
	if r.monitor != nil {
		r.monitor.Close()
	}
	if r.loadRank != nil {
		r.loadRank.Close()
	}
	if r.jobs != nil {
		r.jobs.Close()
	}
	if r.loginLogs != nil {
		r.loginLogs.Close()
	}
}

// LoadRankSampler exposes the HTTP metric sampler for wiring the request log
// listener to the aggregation pipeline.
func (r *Runtime) LoadRankSampler() *loadrank.Sampler {
	if r == nil {
		return nil
	}
	return r.loadRank
}

func Register(r *gin.Engine, conn db.Connection) (*Runtime, error) {
	s := &Store{
		conn: conn,
		auth: newAuthServiceFromEnv(),
	}
	s.security = newSecurityService(s.auth)
	if err := s.syncDefaultPermissions(); err != nil {
		return nil, fmt.Errorf("同步默认权限失败: %w", err)
	}
	if err := s.syncDefaultMenus(); err != nil {
		return nil, fmt.Errorf("同步默认菜单失败: %w", err)
	}
	s.audit = newBusinessAuditRecorder(conn)

	api := r.Group("/api", cors(), s.idempotencyMiddleware(), s.businessAuditMiddleware())
	api.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	loginLogManager, err := loginlogs.Register(api, loginlogs.Dependencies{
		Connection:        s.conn,
		RequireAuth:       s.requireAuth(),
		RequirePermission: s.requirePermission,
	})
	if err != nil {
		s.audit.Close()
		return nil, fmt.Errorf("初始化登录审计失败: %w", err)
	}
	s.loginLogs = loginLogManager

	manager, err := jobs.Register(api, jobs.Dependencies{
		Connection:        s.conn,
		RequireAuth:       s.requireAuth(),
		RequirePermission: s.requirePermission,
		RefreshCache:      s.refreshBuiltInCache,
	})
	if err != nil {
		loginLogManager.Close()
		s.audit.Close()
		return nil, fmt.Errorf("初始化定时任务失败: %w", err)
	}
	s.jobs = manager
	monitorManager, err := monitor.Register(api, monitor.Dependencies{
		Connection:        s.conn,
		RequireAuth:       s.requireAuth(),
		RequirePermission: s.requirePermission,
	})
	if err != nil {
		manager.Close()
		loginLogManager.Close()
		s.audit.Close()
		return nil, fmt.Errorf("初始化系统监控失败: %w", err)
	}
	s.monitor = monitorManager
	loadRankSampler, err := loadrank.Register(api, loadrank.Dependencies{
		Connection:        s.conn,
		RequireAuth:       s.requireAuth(),
		RequirePermission: s.requirePermission,
	})
	if err != nil {
		monitorManager.Close()
		manager.Close()
		loginLogManager.Close()
		s.audit.Close()
		return nil, fmt.Errorf("初始化接口负载排行失败: %w", err)
	}
	s.loadRank = loadRankSampler
	if err := registerApplicationRoutes(api, s); err != nil {
		loadRankSampler.Close()
		monitorManager.Close()
		manager.Close()
		loginLogManager.Close()
		s.audit.Close()
		return nil, fmt.Errorf("初始化应用路由失败: %w", err)
	}
	// Snapshot the route templates after every route is registered so the
	// sampler groups requests by registered template instead of raw paths.
	loadRankSampler.SetRouteIndex(loadrank.NewRouteIndex(r.Routes()))
	return &Runtime{
		audit: s.audit, jobs: manager, loginLogs: loginLogManager,
		monitor: monitorManager, loadRank: loadRankSampler,
	}, nil
}

func registerApplicationRoutes(api *gin.RouterGroup, s *Store) error {
	registerAuthRoutes(api, s)
	registerUserRoutes(api, s)
	registerMenuRoutes(api, s)
	registerRBACRoutes(api, s)
	registerMenuManagementRoutes(api, s)
	registerUserManagementRoutes(api, s)
	registerDictionaryRoutes(api, s)
	files.Register(api, files.Dependencies{
		Connection:        s.conn,
		RequireAuth:       s.requireAuth(),
		RequireAdmin:      s.requireAdmin(),
		RequirePermission: s.requirePermission,
	})
	registerSystemConfigRoutes(api, s)
	registerLogRoutes(api, s)
	if err := codegen.Register(api, codegen.Dependencies{
		Connection:        s.conn,
		RequireAuth:       s.requireAuth(),
		RequirePermission: s.requirePermission,
	}); err != nil {
		return fmt.Errorf("初始化代码生成失败: %w", err)
	}
	if err := generated.RegisterAll(api, generated.Dependencies{
		Connection:        s.conn,
		RequireAuth:       s.requireAuth(),
		RequirePermission: s.requirePermission,
		RegisterAuditResource: func(prefix, resource string, loader func(string) interface{}) {
			RegisterBusinessAuditResource(prefix, resource, AuditSnapshotLoader(loader))
		},
		RegisterIdempotentRoute: RegisterIdempotentCreateRoute,
	}); err != nil {
		return fmt.Errorf("初始化生成模块失败: %w", err)
	}
	if err := notifications.Register(api, notifications.Dependencies{
		Connection:        s.conn,
		RequireAuth:       s.requireAuth(),
		RequirePermission: s.requirePermission,
	}); err != nil {
		return fmt.Errorf("初始化站内通知失败: %w", err)
	}
	if s.loginLogs == nil {
		loginlogs.RegisterRoutes(api, nil, loginlogs.Dependencies{
			RequireAuth:       s.requireAuth(),
			RequirePermission: s.requirePermission,
		})
	}
	if s.jobs == nil {
		jobs.RegisterRoutes(api, nil, jobs.Dependencies{
			RequireAuth:       s.requireAuth(),
			RequirePermission: s.requirePermission,
		})
	}
	if s.monitor == nil {
		monitor.RegisterRoutes(api, nil, monitor.Dependencies{
			RequireAuth:       s.requireAuth(),
			RequirePermission: s.requirePermission,
		})
	}
	if s.loadRank == nil {
		loadrank.RegisterRoutes(api, nil, loadrank.Dependencies{
			RequireAuth:       s.requireAuth(),
			RequirePermission: s.requirePermission,
		})
	}
	return nil
}

func (s *Store) refreshBuiltInCache() error {
	if err := s.syncDefaultPermissions(); err != nil {
		return err
	}
	s.menuMutationMu.Lock()
	defer s.menuMutationMu.Unlock()
	return s.syncDefaultMenus()
}
