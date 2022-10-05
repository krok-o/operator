// Code generated by counterfeiter. DO NOT EDIT.
package providersfakes

import (
	"context"
	"net/http"
	"sync"

	"github.com/krok-o/operator/api/v1alpha1"
	"github.com/krok-o/operator/pkg/providers"
)

type FakePlatform struct {
	CheckoutCodeStub        func(context.Context, *v1alpha1.KrokEvent, *v1alpha1.KrokRepository, string) (string, error)
	checkoutCodeMutex       sync.RWMutex
	checkoutCodeArgsForCall []struct {
		arg1 context.Context
		arg2 *v1alpha1.KrokEvent
		arg3 *v1alpha1.KrokRepository
		arg4 string
	}
	checkoutCodeReturns struct {
		result1 string
		result2 error
	}
	checkoutCodeReturnsOnCall map[int]struct {
		result1 string
		result2 error
	}
	CreateHookStub        func(context.Context, *v1alpha1.KrokRepository, string, string) error
	createHookMutex       sync.RWMutex
	createHookArgsForCall []struct {
		arg1 context.Context
		arg2 *v1alpha1.KrokRepository
		arg3 string
		arg4 string
	}
	createHookReturns struct {
		result1 error
	}
	createHookReturnsOnCall map[int]struct {
		result1 error
	}
	GetEventIDStub        func(context.Context, *http.Request) (string, error)
	getEventIDMutex       sync.RWMutex
	getEventIDArgsForCall []struct {
		arg1 context.Context
		arg2 *http.Request
	}
	getEventIDReturns struct {
		result1 string
		result2 error
	}
	getEventIDReturnsOnCall map[int]struct {
		result1 string
		result2 error
	}
	GetEventTypeStub        func(context.Context, *http.Request) (string, error)
	getEventTypeMutex       sync.RWMutex
	getEventTypeArgsForCall []struct {
		arg1 context.Context
		arg2 *http.Request
	}
	getEventTypeReturns struct {
		result1 string
		result2 error
	}
	getEventTypeReturnsOnCall map[int]struct {
		result1 string
		result2 error
	}
	GetRefIfPresentStub        func(context.Context, *v1alpha1.KrokEvent) (string, string, error)
	getRefIfPresentMutex       sync.RWMutex
	getRefIfPresentArgsForCall []struct {
		arg1 context.Context
		arg2 *v1alpha1.KrokEvent
	}
	getRefIfPresentReturns struct {
		result1 string
		result2 string
		result3 error
	}
	getRefIfPresentReturnsOnCall map[int]struct {
		result1 string
		result2 string
		result3 error
	}
	ValidateRequestStub        func(context.Context, *http.Request, string) (bool, error)
	validateRequestMutex       sync.RWMutex
	validateRequestArgsForCall []struct {
		arg1 context.Context
		arg2 *http.Request
		arg3 string
	}
	validateRequestReturns struct {
		result1 bool
		result2 error
	}
	validateRequestReturnsOnCall map[int]struct {
		result1 bool
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakePlatform) CheckoutCode(arg1 context.Context, arg2 *v1alpha1.KrokEvent, arg3 *v1alpha1.KrokRepository, arg4 string) (string, error) {
	fake.checkoutCodeMutex.Lock()
	ret, specificReturn := fake.checkoutCodeReturnsOnCall[len(fake.checkoutCodeArgsForCall)]
	fake.checkoutCodeArgsForCall = append(fake.checkoutCodeArgsForCall, struct {
		arg1 context.Context
		arg2 *v1alpha1.KrokEvent
		arg3 *v1alpha1.KrokRepository
		arg4 string
	}{arg1, arg2, arg3, arg4})
	stub := fake.CheckoutCodeStub
	fakeReturns := fake.checkoutCodeReturns
	fake.recordInvocation("CheckoutCode", []interface{}{arg1, arg2, arg3, arg4})
	fake.checkoutCodeMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePlatform) CheckoutCodeCallCount() int {
	fake.checkoutCodeMutex.RLock()
	defer fake.checkoutCodeMutex.RUnlock()
	return len(fake.checkoutCodeArgsForCall)
}

func (fake *FakePlatform) CheckoutCodeCalls(stub func(context.Context, *v1alpha1.KrokEvent, *v1alpha1.KrokRepository, string) (string, error)) {
	fake.checkoutCodeMutex.Lock()
	defer fake.checkoutCodeMutex.Unlock()
	fake.CheckoutCodeStub = stub
}

func (fake *FakePlatform) CheckoutCodeArgsForCall(i int) (context.Context, *v1alpha1.KrokEvent, *v1alpha1.KrokRepository, string) {
	fake.checkoutCodeMutex.RLock()
	defer fake.checkoutCodeMutex.RUnlock()
	argsForCall := fake.checkoutCodeArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakePlatform) CheckoutCodeReturns(result1 string, result2 error) {
	fake.checkoutCodeMutex.Lock()
	defer fake.checkoutCodeMutex.Unlock()
	fake.CheckoutCodeStub = nil
	fake.checkoutCodeReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakePlatform) CheckoutCodeReturnsOnCall(i int, result1 string, result2 error) {
	fake.checkoutCodeMutex.Lock()
	defer fake.checkoutCodeMutex.Unlock()
	fake.CheckoutCodeStub = nil
	if fake.checkoutCodeReturnsOnCall == nil {
		fake.checkoutCodeReturnsOnCall = make(map[int]struct {
			result1 string
			result2 error
		})
	}
	fake.checkoutCodeReturnsOnCall[i] = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakePlatform) CreateHook(arg1 context.Context, arg2 *v1alpha1.KrokRepository, arg3 string, arg4 string) error {
	fake.createHookMutex.Lock()
	ret, specificReturn := fake.createHookReturnsOnCall[len(fake.createHookArgsForCall)]
	fake.createHookArgsForCall = append(fake.createHookArgsForCall, struct {
		arg1 context.Context
		arg2 *v1alpha1.KrokRepository
		arg3 string
		arg4 string
	}{arg1, arg2, arg3, arg4})
	stub := fake.CreateHookStub
	fakeReturns := fake.createHookReturns
	fake.recordInvocation("CreateHook", []interface{}{arg1, arg2, arg3, arg4})
	fake.createHookMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakePlatform) CreateHookCallCount() int {
	fake.createHookMutex.RLock()
	defer fake.createHookMutex.RUnlock()
	return len(fake.createHookArgsForCall)
}

func (fake *FakePlatform) CreateHookCalls(stub func(context.Context, *v1alpha1.KrokRepository, string, string) error) {
	fake.createHookMutex.Lock()
	defer fake.createHookMutex.Unlock()
	fake.CreateHookStub = stub
}

func (fake *FakePlatform) CreateHookArgsForCall(i int) (context.Context, *v1alpha1.KrokRepository, string, string) {
	fake.createHookMutex.RLock()
	defer fake.createHookMutex.RUnlock()
	argsForCall := fake.createHookArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakePlatform) CreateHookReturns(result1 error) {
	fake.createHookMutex.Lock()
	defer fake.createHookMutex.Unlock()
	fake.CreateHookStub = nil
	fake.createHookReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakePlatform) CreateHookReturnsOnCall(i int, result1 error) {
	fake.createHookMutex.Lock()
	defer fake.createHookMutex.Unlock()
	fake.CreateHookStub = nil
	if fake.createHookReturnsOnCall == nil {
		fake.createHookReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.createHookReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakePlatform) GetEventID(arg1 context.Context, arg2 *http.Request) (string, error) {
	fake.getEventIDMutex.Lock()
	ret, specificReturn := fake.getEventIDReturnsOnCall[len(fake.getEventIDArgsForCall)]
	fake.getEventIDArgsForCall = append(fake.getEventIDArgsForCall, struct {
		arg1 context.Context
		arg2 *http.Request
	}{arg1, arg2})
	stub := fake.GetEventIDStub
	fakeReturns := fake.getEventIDReturns
	fake.recordInvocation("GetEventID", []interface{}{arg1, arg2})
	fake.getEventIDMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePlatform) GetEventIDCallCount() int {
	fake.getEventIDMutex.RLock()
	defer fake.getEventIDMutex.RUnlock()
	return len(fake.getEventIDArgsForCall)
}

func (fake *FakePlatform) GetEventIDCalls(stub func(context.Context, *http.Request) (string, error)) {
	fake.getEventIDMutex.Lock()
	defer fake.getEventIDMutex.Unlock()
	fake.GetEventIDStub = stub
}

func (fake *FakePlatform) GetEventIDArgsForCall(i int) (context.Context, *http.Request) {
	fake.getEventIDMutex.RLock()
	defer fake.getEventIDMutex.RUnlock()
	argsForCall := fake.getEventIDArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakePlatform) GetEventIDReturns(result1 string, result2 error) {
	fake.getEventIDMutex.Lock()
	defer fake.getEventIDMutex.Unlock()
	fake.GetEventIDStub = nil
	fake.getEventIDReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakePlatform) GetEventIDReturnsOnCall(i int, result1 string, result2 error) {
	fake.getEventIDMutex.Lock()
	defer fake.getEventIDMutex.Unlock()
	fake.GetEventIDStub = nil
	if fake.getEventIDReturnsOnCall == nil {
		fake.getEventIDReturnsOnCall = make(map[int]struct {
			result1 string
			result2 error
		})
	}
	fake.getEventIDReturnsOnCall[i] = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakePlatform) GetEventType(arg1 context.Context, arg2 *http.Request) (string, error) {
	fake.getEventTypeMutex.Lock()
	ret, specificReturn := fake.getEventTypeReturnsOnCall[len(fake.getEventTypeArgsForCall)]
	fake.getEventTypeArgsForCall = append(fake.getEventTypeArgsForCall, struct {
		arg1 context.Context
		arg2 *http.Request
	}{arg1, arg2})
	stub := fake.GetEventTypeStub
	fakeReturns := fake.getEventTypeReturns
	fake.recordInvocation("GetEventType", []interface{}{arg1, arg2})
	fake.getEventTypeMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePlatform) GetEventTypeCallCount() int {
	fake.getEventTypeMutex.RLock()
	defer fake.getEventTypeMutex.RUnlock()
	return len(fake.getEventTypeArgsForCall)
}

func (fake *FakePlatform) GetEventTypeCalls(stub func(context.Context, *http.Request) (string, error)) {
	fake.getEventTypeMutex.Lock()
	defer fake.getEventTypeMutex.Unlock()
	fake.GetEventTypeStub = stub
}

func (fake *FakePlatform) GetEventTypeArgsForCall(i int) (context.Context, *http.Request) {
	fake.getEventTypeMutex.RLock()
	defer fake.getEventTypeMutex.RUnlock()
	argsForCall := fake.getEventTypeArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakePlatform) GetEventTypeReturns(result1 string, result2 error) {
	fake.getEventTypeMutex.Lock()
	defer fake.getEventTypeMutex.Unlock()
	fake.GetEventTypeStub = nil
	fake.getEventTypeReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakePlatform) GetEventTypeReturnsOnCall(i int, result1 string, result2 error) {
	fake.getEventTypeMutex.Lock()
	defer fake.getEventTypeMutex.Unlock()
	fake.GetEventTypeStub = nil
	if fake.getEventTypeReturnsOnCall == nil {
		fake.getEventTypeReturnsOnCall = make(map[int]struct {
			result1 string
			result2 error
		})
	}
	fake.getEventTypeReturnsOnCall[i] = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakePlatform) GetRefIfPresent(arg1 context.Context, arg2 *v1alpha1.KrokEvent) (string, string, error) {
	fake.getRefIfPresentMutex.Lock()
	ret, specificReturn := fake.getRefIfPresentReturnsOnCall[len(fake.getRefIfPresentArgsForCall)]
	fake.getRefIfPresentArgsForCall = append(fake.getRefIfPresentArgsForCall, struct {
		arg1 context.Context
		arg2 *v1alpha1.KrokEvent
	}{arg1, arg2})
	stub := fake.GetRefIfPresentStub
	fakeReturns := fake.getRefIfPresentReturns
	fake.recordInvocation("GetRefIfPresent", []interface{}{arg1, arg2})
	fake.getRefIfPresentMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3
}

func (fake *FakePlatform) GetRefIfPresentCallCount() int {
	fake.getRefIfPresentMutex.RLock()
	defer fake.getRefIfPresentMutex.RUnlock()
	return len(fake.getRefIfPresentArgsForCall)
}

func (fake *FakePlatform) GetRefIfPresentCalls(stub func(context.Context, *v1alpha1.KrokEvent) (string, string, error)) {
	fake.getRefIfPresentMutex.Lock()
	defer fake.getRefIfPresentMutex.Unlock()
	fake.GetRefIfPresentStub = stub
}

func (fake *FakePlatform) GetRefIfPresentArgsForCall(i int) (context.Context, *v1alpha1.KrokEvent) {
	fake.getRefIfPresentMutex.RLock()
	defer fake.getRefIfPresentMutex.RUnlock()
	argsForCall := fake.getRefIfPresentArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakePlatform) GetRefIfPresentReturns(result1 string, result2 string, result3 error) {
	fake.getRefIfPresentMutex.Lock()
	defer fake.getRefIfPresentMutex.Unlock()
	fake.GetRefIfPresentStub = nil
	fake.getRefIfPresentReturns = struct {
		result1 string
		result2 string
		result3 error
	}{result1, result2, result3}
}

func (fake *FakePlatform) GetRefIfPresentReturnsOnCall(i int, result1 string, result2 string, result3 error) {
	fake.getRefIfPresentMutex.Lock()
	defer fake.getRefIfPresentMutex.Unlock()
	fake.GetRefIfPresentStub = nil
	if fake.getRefIfPresentReturnsOnCall == nil {
		fake.getRefIfPresentReturnsOnCall = make(map[int]struct {
			result1 string
			result2 string
			result3 error
		})
	}
	fake.getRefIfPresentReturnsOnCall[i] = struct {
		result1 string
		result2 string
		result3 error
	}{result1, result2, result3}
}

func (fake *FakePlatform) ValidateRequest(arg1 context.Context, arg2 *http.Request, arg3 string) (bool, error) {
	fake.validateRequestMutex.Lock()
	ret, specificReturn := fake.validateRequestReturnsOnCall[len(fake.validateRequestArgsForCall)]
	fake.validateRequestArgsForCall = append(fake.validateRequestArgsForCall, struct {
		arg1 context.Context
		arg2 *http.Request
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.ValidateRequestStub
	fakeReturns := fake.validateRequestReturns
	fake.recordInvocation("ValidateRequest", []interface{}{arg1, arg2, arg3})
	fake.validateRequestMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePlatform) ValidateRequestCallCount() int {
	fake.validateRequestMutex.RLock()
	defer fake.validateRequestMutex.RUnlock()
	return len(fake.validateRequestArgsForCall)
}

func (fake *FakePlatform) ValidateRequestCalls(stub func(context.Context, *http.Request, string) (bool, error)) {
	fake.validateRequestMutex.Lock()
	defer fake.validateRequestMutex.Unlock()
	fake.ValidateRequestStub = stub
}

func (fake *FakePlatform) ValidateRequestArgsForCall(i int) (context.Context, *http.Request, string) {
	fake.validateRequestMutex.RLock()
	defer fake.validateRequestMutex.RUnlock()
	argsForCall := fake.validateRequestArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakePlatform) ValidateRequestReturns(result1 bool, result2 error) {
	fake.validateRequestMutex.Lock()
	defer fake.validateRequestMutex.Unlock()
	fake.ValidateRequestStub = nil
	fake.validateRequestReturns = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePlatform) ValidateRequestReturnsOnCall(i int, result1 bool, result2 error) {
	fake.validateRequestMutex.Lock()
	defer fake.validateRequestMutex.Unlock()
	fake.ValidateRequestStub = nil
	if fake.validateRequestReturnsOnCall == nil {
		fake.validateRequestReturnsOnCall = make(map[int]struct {
			result1 bool
			result2 error
		})
	}
	fake.validateRequestReturnsOnCall[i] = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePlatform) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.checkoutCodeMutex.RLock()
	defer fake.checkoutCodeMutex.RUnlock()
	fake.createHookMutex.RLock()
	defer fake.createHookMutex.RUnlock()
	fake.getEventIDMutex.RLock()
	defer fake.getEventIDMutex.RUnlock()
	fake.getEventTypeMutex.RLock()
	defer fake.getEventTypeMutex.RUnlock()
	fake.getRefIfPresentMutex.RLock()
	defer fake.getRefIfPresentMutex.RUnlock()
	fake.validateRequestMutex.RLock()
	defer fake.validateRequestMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakePlatform) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ providers.Platform = new(FakePlatform)
