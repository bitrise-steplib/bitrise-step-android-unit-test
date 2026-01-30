# Android Unit Test

[![Step changelog](https://shields.io/github/v/release/bitrise-steplib/bitrise-step-android-unit-test?include_prereleases&label=changelog&color=blueviolet)](https://github.com/bitrise-steplib/bitrise-step-android-unit-test/releases)

This step runs your Android project's unit tests.

<details>
<summary>Description</summary>

This step runs your Android project's unit tests.
</details>

## 🧩 Get started

Add this step directly to your workflow in the [Bitrise Workflow Editor](https://docs.bitrise.io/en/bitrise-ci/workflows-and-pipelines/steps/adding-steps-to-a-workflow.html).

You can also run this step directly with [Bitrise CLI](https://github.com/bitrise-io/bitrise).

## ⚙️ Configuration

<details>
<summary>Inputs</summary>

| Key | Description | Flags | Default |
| --- | --- | --- | --- |
| `project_location` | The root directory of your android project, for example, where your root build gradle file exists (also gradlew, settings.gradle, etc...) | required | `$BITRISE_SOURCE_DIR` |
| `module` | Set the module that you want to test. To see your available modules, please open your project in Android Studio, go to **Project Structure** and see the list on the left. Leave this input blank to test all modules.  |  |  |
| `variant` | Set the variant that you want to test. To see your available variants, please open your project in Android Studio, go to **Project Structure**, then to the **variants** section. Leave this input blank to test all variants.  |  |  |
| `arguments` | Extra arguments passed to the gradle task |  |  |
| `report_path_pattern` | The step will use this pattern to export __Local unit test HTML results__. The whole HTML results directory will be zipped and moved to the `$BITRISE_DEPLOY_DIR`.  You need to override this input if you have custom output dir set for Local unit test HTML results. The pattern needs to be relative to the selected module's directory.  Example 1: app module and debug variant is selected and the HTML report is generated at:  - `<path_to_your_project>/app/build/reports/tests/testDebugUnitTest`  this case use: `*build/reports/tests/testDebugUnitTest` pattern.  Example 2: app module and NO variant is selected and the HTML reports are generated at:  - `<path_to_your_project>/app/build/reports/tests/testDebugUnitTest` - `<path_to_your_project>/app/build/reports/tests/testReleaseUnitTest`  to export every variant's reports use: `*build/reports/tests` pattern. | required | `*build/reports/tests` |
| `result_path_pattern` | The step will use this pattern to export __Local unit test XML results__. The whole XML results directory will be zipped and moved to the `$BITRISE_DEPLOY_DIR` and the result files will be deployed to the Ship Addon.  You need to override this input if you have custom output dir set for Local unit test XML results. The pattern needs to be relative to the selected module's directory.  Example 1: app module and debug variant is selected and the XML report is generated at:  - `<path_to_your_project>/app/build/test-results/testDebugUnitTest`  this case use: `*build/test-results/testDebugUnitTest` pattern.  Example 2: app module and NO variant is selected and the XML reports are generated at:  - `<path_to_your_project>/app/build/test-results/testDebugUnitTest` - `<path_to_your_project>/app/build/test-results/testReleaseUnitTest`  to export every variant's reports use: `*build/test-results` pattern. | required | `*build/test-results` |
| `is_debug` | The step will print more verbose logs if enabled. | required | `false` |
| `quarantined_tests` | JSON list of tests added to quarantine on Bitrise.io, quarantined tests are excluded from test runs. |  | `$BITRISE_QUARANTINED_TESTS_JSON` |
</details>

<details>
<summary>Outputs</summary>

| Environment Variable | Description |
| --- | --- |
| `BITRISE_FLAKY_TEST_CASES` | A test case is considered flaky if it has failed at least once, but passed at least once as well.  The list contains the test cases in the following format: ``` - TestSuit_1.TestClass_1.TestName_1 - TestSuit_1.TestClass_1.TestName_2 - TestSuit_1.TestClass_2.TestName_1 - TestSuit_2.TestClass_1.TestName_1 ... ``` |
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/bitrise-step-android-unit-test/pulls) and [issues](https://github.com/bitrise-steplib/bitrise-step-android-unit-test/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://docs.bitrise.io/en/bitrise-ci/bitrise-cli/running-your-first-local-build-with-the-cli.html).

Learn more about developing steps:

- [Create your own step](https://docs.bitrise.io/en/bitrise-ci/workflows-and-pipelines/developing-your-own-bitrise-step/developing-a-new-step.html)
