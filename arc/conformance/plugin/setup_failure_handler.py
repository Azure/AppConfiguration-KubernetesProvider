import sys
from junit_xml import TestSuite, TestCase

# Reading error message from error file
with open('/tmp/results/error', 'r') as f:
  error_message = f.read()

# Creating a junit report for setup failure
test_case = TestCase('azure_app_configuration_provider_conformance_setup', 'azure_app_configuration_provider_conformance_setup')
test_case.add_failure_info(error_message)
test_cases = [test_case]
test_suite = TestSuite("azure_app_configuration_provider_conformance", test_cases)

with open('/tmp/results/results.xml', 'w') as f:
  TestSuite.to_file(f, [test_suite], prettyprint=False)

# Exit with non-zero return code
sys.exit(1)
