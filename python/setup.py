from setuptools import setup, find_packages

setup(
    name="cli-llm",
    version="0.1.0",
    packages=find_packages(),
    package_data={
        'cli_llm': ['prompts.py'],
    },
    install_requires=[
        "click>=8.0.0",
        "openai>=1.0.0",
    ],
    entry_points={
        'console_scripts': [
            'llm=llm:main',
        ],
    },
    author="Egg12138",
    author_email="xoltraman@outlook.com",
    description="A simple CLI LLM client for interacting with large language models",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    url="https://github.com/Eggg12138/cli-llm",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
    ],
    python_requires=">=3.8",
) 