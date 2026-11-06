## Problem and changes / 问题与变更

<!-- Describe the problem, main changes and user-visible behavior. / 描述解决的问题、主要变更和用户可见行为。 -->

## Related issues / 关联 Issue

<!-- Use Closes #123 if applicable; an issue is not required for every PR. / 如适用填写 Closes #123；无需为每个 PR 创建 Issue。 -->

## Compatibility and dependencies / 兼容性与依赖

<!-- Explain breaking changes, platform impact and rationale/license compatibility for new direct dependencies, or mark not applicable. / 如适用，说明破坏性变更、平台影响和新增直接依赖的理由与许可证兼容性；否则填不适用。 -->

## Validation / 验证

<!-- List commands actually run and their results; explain skipped or inapplicable checks. / 列出实际执行的命令及结果，未执行或不适用的检查需说明原因。 -->
<!-- For Go changes, follow CONTRIBUTING.md / CONTRIBUTING.zh-CN.md and add checks relevant to the changed surface. / Go 改动参照对应贡献指南，按改动范围补充检查。 -->
<!-- For documentation-only changes, check links, anchors, examples and bilingual coverage; Go tests may be not applicable. Test documentation packaging if paths change. / 纯文档改动检查链接、锚点、示例和双语覆盖，可注明 Go 测试不适用；路径变化需检查文档打包。 -->
<!-- Run integration tests only against a dedicated disposable Mihomo instance, never your daily instance. / 集成测试只能使用专用的一次性 Mihomo 实例，不使用日常实例。 -->

## Checklist / 提交检查

<!-- Explain any inapplicable items. / 不适用的检查项请注明原因。 -->

- [ ] Changes are focused on this PR's problem. / 改动聚焦于本 PR 的问题。
- [ ] Tests were added or updated for behavior changes. / 行为变化已添加或更新测试。
- [ ] User-visible changes are reflected in the English docs and complete Chinese counterparts. / 用户可见变化已同步更新英文文档及完整中文对应版本。
- [ ] Moved documentation has updated links, language switches and release packaging where applicable. / 文档迁移已按需更新链接、语言切换和发布打包。
- [ ] Commit messages follow Conventional Commits. / 提交信息符合 Conventional Commits。
- [ ] Text, examples, configuration, logs and screenshots contain no secrets, subscription URLs, private storage settings or real traffic data. / 文字、示例、配置、日志和截图不含密钥、订阅地址、私有存储配置或真实流量数据。
