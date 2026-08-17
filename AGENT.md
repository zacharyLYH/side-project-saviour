# Coding standards
1. MINIMAL DEPENDENCIES. Fewer dependencies and the more mainstream and battle tested the dependencies, the better. 
2. MINIMAL LINES OF CODE. More lines of code === more complexity. That is evil.
3. JUDGEMENT CALLS SHOULD BE SENSIBLE DEFAULTS. Sometimes users don't spell out every minute detail, sensible defaults is your target when behavior / features are ambiguous. One thing you can ask yourself is "How would Github Codespaces a polished Saas app do this behavior if they had to?"
4. DEFER TESTS UNTIL USER HAS CONFIRMED FEATURE PARITY. Otherwise, you might just be wasting user's time creating tests when the feature might not be ready yet!
5. NO THIN MODULES NOR BIG FUNCTIONS. To know when abstraction is a good idea, ask yourself if reading some abstracted function actually reduces mental friction. 

# Go code
1. INTERFACES AND MOCKS. APIs should be interfaces so that they can be easily mocked using Mockery.
2. TESTS. Unit tests and intg tests are a must. Where it makes sense, use the mockery generated mocks.

# Typescript code
1. TYPED. Everything must be typed. 
2. TESTS. Playwright behavioral and visual tests should be first class from day 1. Mocks are allowed, but push it down as far as possible - we want to exercise as much behavior as possible!