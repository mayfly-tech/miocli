## 问题与变更 / Problem and changes

<!-- 描述解决的问题、主要变更和用户可见行为。 / Describe the problem, main changes and user-visible behavior. -->

## 关联 Issue / Related issues

<!-- 如适用填写 Closes #123；无需为每个 PR 创建 Issue。 / Use Closes #123 if applicable; an issue is not required for every PR. -->

## 兼容性与依赖 / Compatibility and dependencies

<!-- 如适用，说明破坏性变更、平台影响和新增直接依赖的理由与许可证兼容性；否则填不适用。 / Explain breaking changes, platform impact and rationale/license compatibility for new direct dependencies, or mark not applicable. -->

## 验证 / Validation

<!-- 列出实际执行的命令及结果，未执行或不适用的检查需说明原因。 / List commands actually run and their results; explain skipped or inapplicable checks. -->
<!-- Go 改动参考贡献指南中的 gofmt、go vet ./...、go test -race ./... 和 go build -o /tmp/miocli ./cmd/miocli；按改动范围补充检查。 / For Go changes, see the contributing guide for gofmt, go vet ./..., go test -race ./... and go build -o /tmp/miocli ./cmd/miocli; add checks relevant to the changed surface. -->
<!-- 纯文档或模板改动可注明 Go 测试不适用。集成测试只能使用专用的一次性 Mihomo 实例，不使用日常实例。 / For documentation/template-only changes, Go tests may be marked not applicable. Run integration tests only against a dedicated disposable Mihomo instance, never your daily instance. -->

## 提交检查 / Checklist

<!-- 不适用的检查项请注明原因。 / Explain any inapplicable items. -->

- [ ] 改动聚焦于本 PR 的问题。 / Changes are focused on this PR's problem.
- [ ] 行为变化已添加或更新测试。 / Tests were added or updated for behavior changes.
- [ ] 用户可见变化已同步更新中英文 README 和相关指南。 / User-visible changes are reflected in both READMEs and relevant guides.
- [ ] 提交信息符合 Conventional Commits。 / Commit messages follow Conventional Commits.
- [ ] 文字、示例、配置、日志和截图不含密钥、订阅地址、私有存储配置或真实流量数据。 / Text, examples, configuration, logs and screenshots contain no secrets, subscription URLs, private storage settings or real traffic data.
