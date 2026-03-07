Feature: Loop workflows
  As an operator
  I want deterministic loop workflow behavior
  So that I can trust single and concurrent loop execution safety

  Scenario: Single loop completes successfully
    Given a loop workflow fixture is initialized
    When I run single loop completion flow
    Then the loop should reach synced state

  Scenario: Concurrent loops remain isolated
    Given a loop workflow fixture is initialized
    When I run concurrent loop safety flow
    Then each loop should have an isolated lock owner
