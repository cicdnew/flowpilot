# FlowPilot Examples

This directory contains sample configurations and use cases for FlowPilot's repeated task feature.

## Available Examples

### Chhotu.com Domain Testing
- **`chhotu_com_sample.json`**: Complete JSON configuration for repeated task testing
- **`chhotu_com_scenarios.md`**: 7 detailed scenarios with different testing strategies

## How to Use These Examples

### Option 1: Manual Entry via UI
1. Open FlowPilot
2. Click "+ Repeat Task"
3. Copy values from the scenario files
4. Adjust as needed
5. Create tasks

### Option 2: Reference for API Integration
Use these examples as templates when integrating with FlowPilot's API or building automation scripts.

## Scenarios Included

1. **Basic Page Range Test**: Test pages 100-200
2. **Large Scale Test**: Quick smoke test (1-1000, step 10)
3. **Product Page Deep Test**: Comprehensive product testing with interactions
4. **Category Testing**: Test different site sections using list mode
5. **Search Query Testing**: Validate search functionality
6. **User Journey Simulation**: End-to-end flow testing
7. **API Endpoint Testing**: Test API responses

## Customization Tips

- **URL Pattern**: Replace `chhotu.com` with your domain
- **Selectors**: Update CSS selectors to match your site structure
- **Range Values**: Adjust start/end/step based on your data
- **Tags**: Use meaningful tags for organization
- **Priorities**: Higher priority (1-10) for critical tests

## Testing Strategy

Start with small batches (2-3 tasks) to:
- Verify selectors work
- Validate URL patterns
- Test timing/waits
- Check proxy configuration

Then scale up to full ranges once validated.

## Additional Resources

- `../REPEAT_TASK_FEATURE.md` - Complete feature documentation
- `../DEMO_GUIDE.md` - Step-by-step usage guide
- `../IMPLEMENTATION_SUMMARY.md` - Technical implementation details
