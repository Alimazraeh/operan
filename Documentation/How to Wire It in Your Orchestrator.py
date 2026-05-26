# Pseudocode for agent loop
for wave in waves:
    for module in wave:
        # 1. ARCH generates contracts
        ARCH.run(prompt=f"Generate contracts/v1/ and integration-graph for {module}")
        
        # 2. CODER implements (parallel A/B)
        CODER_A.run(prompt=f"Implement {module} against contracts/v1/")
        # CODER_B runs next wave module concurrently
        
        # 3. REVIEW validates
        result = subprocess.run(["python3", "review_validate.py"], env={"MODULE_ID": module})
        
        # 4. Gate or loop
        if result.returncode == 0:
            ARCH.run(prompt=f"Merge {module} to main. Update scaffold.")
        else:
            CODER_A.run(prompt=f"Fix drift in {module}. Review report: reports/{module}-review.md")