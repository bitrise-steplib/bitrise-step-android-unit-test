# Android Unit Test

[![Step changelog](https://shields.io/github/v/release/bitrise-steplib/bitrise-step-android-unit-test?include_prereleases&label=changelog&color=blueviolet)](https://github.com/bitrise-steplib/bitrise-step-android-unit-test/releases)

This step runs your Android project's unit tests.

<details>
<summary>Description</summary>

This step runs your Android project's unit tests.
</details>

## 🧩 Get started

Add this step directly to your workflow in the [Bitrise Workflow Editor](https://devcenter.bitrise.io/steps-and-workflows/steps-and-workflows-index/).

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
</details>

<details>
<summary>Outputs</summary>
There are no outputs defined in this step
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/bitrise-step-android-unit-test/pulls) and [issues](https://github.com/bitrise-steplib/bitrise-step-android-unit-test/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://devcenter.bitrise.io/bitrise-cli/run-your-first-build/).

Learn more about developing steps:

- [Create your own step](https://devcenter.bitrise.io/contributors/create-your-own-step/)
- [Testing your Step](https://devcenter.bitrise.io/contributors/testing-and-versioning-your-steps/)
