package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mison/antigravity-go/internal/tools"
)

const deployCmd = "deploy"

func runDeploy(args []string) {
	fs := flag.NewFlagSet(deployCmd, flag.ExitOnError)
	providerF := fs.String("provider", "", "LLM provider")
	modelF := fs.String("model", "", "Model to use")
	baseURLF := fs.String("base-url", "", "Base URL")
	envF := fs.String("env", "staging", "Deployment environment")
	imageF := fs.String("image", "", "Image repository")
	tagF := fs.String("tag", "", "Image tag")
	previousF := fs.String("previous-image", "", "Previous deployed image reference")
	executeF := fs.Bool("execute", false, "Execute local docker build/push instead of simulation")
	_ = fs.Parse(args)

	if fs.NArg() != 0 {
		fmt.Printf("用法: ago %s [--env staging] [--image repo/app] [--tag v1] [--execute]\n", deployCmd)
		return
	}

	rt, err := newCommandRuntime(*providerF, *modelF, *baseURLF)
	if err != nil {
		fmt.Printf("%s 初始化失败: %v\n", deployCmd, err)
		return
	}
	defer rt.Close()

	plan, err := tools.BuildDeploymentPlan(rt.cwd, tools.DeploymentPlanOptions{
		Environment:      *envF,
		ImageRepository:  *imageF,
		ImageTag:         *tagF,
		PreviousImageRef: *previousF,
	})
	if err != nil {
		fmt.Printf("%s 生成部署计划失败: %v\n", deployCmd, err)
		return
	}
	if err := tools.WriteDeploymentArtifacts(rt.cwd, plan); err != nil {
		fmt.Printf("%s 写入部署交付物失败: %v\n", deployCmd, err)
		return
	}

	manager := tools.NewDeploymentManager(filepath.Join(rt.cfg.DataDir, "deployments"), nil)
	record, err := manager.Prepare(plan)
	if err != nil {
		fmt.Printf("%s 创建部署记录失败: %v\n", deployCmd, err)
		return
	}

	fmt.Printf("[INFO] 部署模式: %s\n", deployModeLabel(*executeF))
	fmt.Printf("[INFO] 镜像: %s\n", plan.ImageRef)
	fmt.Printf("[INFO] 记录: %s\n", record.ID)
	fmt.Printf("[INFO] Workflow: %s\n", plan.GitHubActionPath)

	report, err := runDeployReview(context.Background(), rt, plan)
	if err != nil {
		_ = manager.Rollback(record, "review", err)
		fmt.Printf("%s 预检失败: %v\n", deployCmd, err)
		return
	}
	fmt.Println(report)

	passed := deploymentReviewPassed(report)
	if err := manager.RecordReview(record, report, passed); err != nil {
		fmt.Printf("%s 写入预检结果失败: %v\n", deployCmd, err)
		return
	}
	if !passed {
		_ = manager.Rollback(record, "review", fmt.Errorf("ReviewerAgent precheck failed"))
		fmt.Printf("%s 已中止: ReviewerAgent 未通过\n", deployCmd)
		return
	}

	if err := runDeployStep(context.Background(), manager, record, plan, *executeF); err != nil {
		fmt.Printf("%s 运行失败: %v\n", deployCmd, err)
		return
	}
	fmt.Printf("[OK] 部署完成: %s\n", plan.ImageRef)
}

func runDeployReview(ctx context.Context, rt *commandRuntime, plan tools.DeploymentPlan) (string, error) {
	reviewAgt := rt.newReviewAgent()
	return reviewAgt.Run(ctx, buildDeployReviewPrompt(plan), nil)
}

func buildDeployReviewPrompt(plan tools.DeploymentPlan) string {
	return fmt.Sprintf(`请对以下上线计划执行生产预检，不要修改任何文件。
目标环境: %s
镜像: %s
工作区: %s

Dockerfile:
%s

.dockerignore:
%s

docker-compose.yml:
%s

部署命令:
- %s
- %s

要求:
1. 重点检查容器默认行为、对外暴露面、鉴权要求、敏感信息风险和生产可运维性。
2. 必须调用 ask_specialist(role="reviewer") 汇总最终判断。
3. 最终输出第一行只能是 PASS 或 FAIL，后续最多 5 条发现。`,
		plan.Environment,
		plan.ImageRef,
		plan.WorkspaceRoot,
		plan.DockerfileContent,
		plan.DockerignoreBody,
		plan.DockerComposeBody,
		plan.BuildCommand,
		plan.PushCommand,
	)
}

func deploymentReviewPassed(report string) bool {
	for _, line := range strings.Split(report, "\n") {
		trimmed := strings.ToUpper(strings.TrimSpace(line))
		if trimmed == "" {
			continue
		}
		return trimmed == "PASS"
	}
	return false
}

func runDeployStep(ctx context.Context, manager *tools.DeploymentManager, record *tools.DeploymentRecord, plan tools.DeploymentPlan, execute bool) error {
	buildOutput, err := manager.RunBuild(ctx, record, plan, execute)
	fmt.Println(buildOutput)
	if err != nil {
		_ = manager.Rollback(record, "build", err)
		return err
	}

	pushOutput, err := manager.RunPush(ctx, record, plan, execute)
	fmt.Println(pushOutput)
	if err != nil {
		_ = manager.Rollback(record, "push", err)
		return err
	}
	if err := manager.MarkCommitted(record); err != nil {
		return err
	}
	return nil
}

func deployModeLabel(execute bool) string {
	if execute {
		return "execute"
	}
	return "simulate"
}
